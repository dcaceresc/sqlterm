// Package result provides a disk-backed grid. Only a small page of rows is cached.
package result

import (
	"bufio"
	"context"
	"encoding/binary"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

type Cell struct {
	Text string `json:"text"`
	Null bool   `json:"null,omitempty"`
}
type Store struct {
	mu          sync.Mutex
	dir         string
	data, index *os.File
	writer      *bufio.Writer
	offset      int64
	count       int
	cacheStart  int
	cache       [][]Cell
	Columns     []string
}

func New(columns []string) (*Store, error) {
	dir, e := os.MkdirTemp("", "sqlterm-results-*")
	if e != nil {
		return nil, e
	}
	s := &Store{dir: dir, Columns: columns, cacheStart: -1}
	s.data, e = os.OpenFile(filepath.Join(dir, "rows.jsonl"), os.O_CREATE|os.O_RDWR, 0600)
	if e == nil {
		s.index, e = os.OpenFile(filepath.Join(dir, "offsets"), os.O_CREATE|os.O_RDWR, 0600)
	}
	if e != nil {
		s.Close()
		return nil, e
	}
	s.writer = bufio.NewWriterSize(s.data, 64*1024)
	return s, nil
}
func (s *Store) Append(row []Cell) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.writer == nil {
		return os.ErrClosed
	}
	b, e := json.Marshal(row)
	if e != nil {
		return e
	}
	var offset [8]byte
	binary.LittleEndian.PutUint64(offset[:], uint64(s.offset))
	if _, e = s.index.Write(offset[:]); e != nil {
		return e
	}
	b = append(b, '\n')
	if _, e = s.writer.Write(b); e != nil {
		return e
	}
	s.offset += int64(len(b))
	s.count++
	return nil
}
func (s *Store) Flush() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.writer == nil {
		return os.ErrClosed
	}
	return s.writer.Flush()
}
func (s *Store) Count() int { s.mu.Lock(); defer s.mu.Unlock(); return s.count }
func (s *Store) Row(n int) ([]Cell, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.writer == nil {
		return nil, os.ErrClosed
	}
	if n < 0 || n >= s.count {
		return nil, io.EOF
	}
	if n >= s.cacheStart && n < s.cacheStart+len(s.cache) {
		return s.cache[n-s.cacheStart], nil
	}
	if e := s.writer.Flush(); e != nil {
		return nil, e
	}
	start := n / 128 * 128
	var off [8]byte
	if _, e := s.index.ReadAt(off[:], int64(start)*8); e != nil {
		return nil, e
	}
	r := bufio.NewReader(io.NewSectionReader(s.data, int64(binary.LittleEndian.Uint64(off[:])), s.offset))
	s.cache = nil
	s.cacheStart = start
	for j := start; j < s.count && j < start+128; j++ {
		b, e := r.ReadBytes('\n')
		if e != nil {
			return nil, e
		}
		var row []Cell
		if e = json.Unmarshal(b, &row); e != nil {
			return nil, e
		}
		s.cache = append(s.cache, row)
	}
	return s.cache[n-start], nil
}
func (s *Store) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.writer != nil {
		s.writer.Flush()
	}
	if s.data != nil {
		s.data.Close()
	}
	if s.index != nil {
		s.index.Close()
	}
	if s.dir != "" {
		os.RemoveAll(s.dir)
	}
	s.writer = nil
	s.data = nil
	s.index = nil
	s.dir = ""
	s.cache = nil
}
func (s *Store) Export(path, format string) error {
	return s.ExportContext(context.Background(), path, format)
}

// ExportContext preserves literal CSV text and JSON for existing consumers.
func (s *Store) ExportContext(ctx context.Context, path, format string) error {
	return s.exportContext(ctx, path, format, false)
}

// ExportSpreadsheetContext makes CSV fields safe to open in spreadsheet apps.
// JSON remains unchanged because it is not a spreadsheet-oriented format.
func (s *Store) ExportSpreadsheetContext(ctx context.Context, path, format string) error {
	return s.exportContext(ctx, path, format, true)
}

func (s *Store) exportContext(ctx context.Context, path, format string, spreadsheetSafe bool) error {
	f, e := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if e != nil {
		return e
	}
	success := false
	defer func() {
		f.Close()
		if !success {
			os.Remove(path)
		}
	}()
	w := bufio.NewWriter(f)
	switch format {
	case "csv":
		c := csv.NewWriter(w)
		columns := s.Columns
		if spreadsheetSafe {
			columns = spreadsheetSafeRecord(columns)
		}
		if e = c.Write(columns); e != nil {
			return e
		}
		for i := 0; i < s.Count(); i++ {
			if e = ctx.Err(); e != nil {
				return e
			}
			r, e := s.Row(i)
			if e != nil {
				return e
			}
			v := make([]string, len(r))
			for j, x := range r {
				if x.Null {
					v[j] = `\N`
				} else {
					v[j] = x.Text
					if strings.HasPrefix(v[j], `\`) {
						v[j] = `\` + v[j]
					}
				}
			}
			if spreadsheetSafe {
				v = spreadsheetSafeRecord(v)
			}
			if len(v) == 1 && v[0] == "" {
				c.Flush()
				if e = c.Error(); e != nil {
					return e
				}
				if _, e = w.WriteString("\"\"\n"); e != nil {
					return e
				}
				continue
			}
			if e = c.Write(v); e != nil {
				return e
			}
		}
		c.Flush()
		if e = c.Error(); e != nil {
			return e
		}
	case "json":
		// A column array plus row arrays preserves duplicate column names and SQL NULL.
		cols, _ := json.Marshal(s.Columns)
		if _, e = fmt.Fprintf(w, "{\"columns\":%s,\"rows\":[", cols); e != nil {
			return e
		}
		for i := 0; i < s.Count(); i++ {
			if e = ctx.Err(); e != nil {
				return e
			}
			r, e := s.Row(i)
			if e != nil {
				return e
			}
			v := make([]any, len(r))
			for j, x := range r {
				if !x.Null {
					v[j] = x.Text
				}
			}
			b, e := json.Marshal(v)
			if e != nil {
				return e
			}
			if i > 0 {
				if e = w.WriteByte(','); e != nil {
					return e
				}
			}
			if _, e = w.Write(b); e != nil {
				return e
			}
		}
		if _, e = w.WriteString("]}\n"); e != nil {
			return e
		}
	default:
		return fmt.Errorf("format must be csv or json")
	}
	if e = w.Flush(); e != nil {
		return e
	}
	if e = f.Close(); e != nil {
		return e
	}
	success = true
	return nil
}

// Order returns a local ordering without retaining cell values in memory.
// Row indexes alone are retained; stable comparisons load rows through the cache.
func (s *Store) Order(column int, descending bool) ([]int, error) {
	return s.OrderContext(context.Background(), column, descending)
}
func (s *Store) OrderContext(ctx context.Context, column int, descending bool) ([]int, error) {
	if column < 0 || column >= len(s.Columns) {
		return nil, fmt.Errorf("invalid column")
	}
	ids := make([]int, s.Count())
	for i := range ids {
		ids[i] = i
	}
	var failure error
	sort.SliceStable(ids, func(i, j int) bool {
		if failure != nil {
			return false
		}
		if e := ctx.Err(); e != nil {
			failure = e
			return false
		}
		a, e := s.Row(ids[i])
		if e != nil {
			failure = e
			return false
		}
		b, e := s.Row(ids[j])
		if e != nil {
			failure = e
			return false
		}
		x, y := a[column], b[column]
		cmp := strings.Compare(x.Text, y.Text)
		if x.Null != y.Null {
			if x.Null {
				cmp = -1
			} else {
				cmp = 1
			}
		}
		if descending {
			return cmp > 0
		}
		return cmp < 0
	})
	return ids, failure
}
func (s *Store) Find(ctxDone <-chan struct{}, text string, start int) (int, error) {
	for i := 0; i < s.Count(); i++ {
		select {
		case <-ctxDone:
			return -1, fmt.Errorf("search canceled")
		default:
		}
		n := (start + i) % s.Count()
		r, e := s.Row(n)
		if e != nil {
			return -1, e
		}
		for _, v := range r {
			if strings.Contains(strings.ToLower(v.Text), strings.ToLower(text)) {
				return n, nil
			}
		}
	}
	return -1, nil
}
