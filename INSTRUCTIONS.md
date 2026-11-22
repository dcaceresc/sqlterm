# Instrucciones de SQLTerm

Esta guía explica cómo instalar, configurar y usar SQLTerm. El cliente está orientado al flujo personal para el que fue creado, tiene Linux y Windows como plataformas principales y admite SQL Server, MySQL y PostgreSQL.

## Contenido

- [Instalación recomendada](#instalación-recomendada)
- [Automatización local](#automatización-local)
- [Primer flujo de trabajo](#primer-flujo-de-trabajo)
- [Editor y archivos SQL](#editor-y-archivos-sql)
- [Conexiones, perfiles y bases de datos](#conexiones-perfiles-y-bases-de-datos)
- [Catálogo, tablas y rutinas](#catálogo-tablas-y-rutinas)
- [Ejecución y confirmaciones](#ejecución-y-confirmaciones)
- [Resultados, copia y exportación](#resultados-copia-y-exportación)
- [IntelliSense](#intellisense)
- [Interfaz, temas y carpetas](#interfaz-temas-y-carpetas)
- [Atajos](#atajos)
- [Configuración y credenciales](#configuración-y-credenciales)
- [Registro de errores e historial](#registro-de-errores-e-historial)
- [Scripts y comportamiento por motor](#scripts-y-comportamiento-por-motor)
- [Pruebas](#pruebas)
- [Límites conocidos](#límites-conocidos)
- [Estructura del repositorio](#estructura-del-repositorio)

## Instalación recomendada

Instala SQLTerm una vez desde la raíz del repositorio y después ejecútalo desde cualquier carpeta. Go 1.24 o posterior solo es necesario para compilar o instalar desde el código fuente. El binario instalado no necesita Go, pero sí **Neovim 0.11 o posterior**, disponible como `nvim` en `PATH`. Comprueba la dependencia con `nvim --version` antes de iniciar SQLTerm.

```sh
go install ./cmd/sqlterm
sqlterm --version
```

`go install` escribe el ejecutable en `GOBIN` cuando está configurado. En caso contrario, usa el subdirectorio `bin` de la primera entrada de `GOPATH`. Consulta los valores reales de tu entorno con:

```sh
go env GOBIN
go env GOPATH
```

Con el destino de usuario predeterminado, la instalación no requiere `sudo` ni permisos de administrador. Si configuraste otro `GOBIN`, necesitas permiso de escritura en ese directorio. La instalación puede reemplazar una versión anterior de `sqlterm` en el mismo destino. El comando no agrega ni persiste automáticamente ese directorio en `PATH`.

### Linux

El ejecutable se llama `sqlterm`. Si el directorio de instalación todavía no está en `PATH`, este ejemplo determina `GOBIN` o la primera entrada de `GOPATH` y lo agrega solo a la sesión actual, sin reemplazar la configuración ni el resto de `PATH`:

```sh
gobin=$(go env GOBIN)
if [ -n "$gobin" ]; then
  install_dir=$gobin
else
  gopath=$(go env GOPATH)
  install_dir=${gopath%%:*}/bin
fi
case ":$PATH:" in
  *":$install_dir:"*) ;;
  *) export PATH="$install_dir:$PATH" ;;
esac
sqlterm --version
```

Para conservarlo en terminales futuras, agrega por tu cuenta una instrucción con el directorio real, por ejemplo `export PATH="/ruta/real/bin:$PATH"`, al archivo de inicio de tu shell de usuario. No reemplaces el archivo ni modifiques la configuración global del sistema.

En sesiones gráficas X11, copiar al portapapeles puede requerir `xclip` o `xsel`. Guardar credenciales puede requerir Secret Service y una sesión que exponga ese servicio.

### Windows

El ejecutable instalado se llama `sqlterm.exe`, pero se invoca como `sqlterm`. Localiza el directorio con `go env GOBIN` o, si está vacío, usa `bin` bajo la primera entrada mostrada por `go env GOPATH`. Agrega ese directorio al `Path` del usuario desde la configuración de variables de entorno de Windows y vuelve a abrir la terminal; no es necesario modificar el `Path` del sistema ni usar privilegios de administrador.

SQLTerm usa los servicios de Windows disponibles para el portapapeles y el almacén de credenciales. Su disponibilidad también depende de la sesión en la que se ejecute.

### Usar un binario existente sin Go

También puedes colocar en un directorio de tu `PATH` un binario de SQLTerm que corresponda exactamente a tu sistema y arquitectura. Para ejecutarlo no necesitas Go. El repositorio puede generar esos binarios, pero esta guía no presupone que exista una descarga pública.

Para actualizar una instalación hecha desde código, actualiza primero tu copia del repositorio por el medio que elijas y vuelve a ejecutar `go install ./cmd/sqlterm` desde su raíz. SQLTerm no descarga cambios ni actualiza el repositorio automáticamente.

### Alternativas para desarrollo

Para una ejecución puntual desde la raíz del repositorio:

```sh
go run ./cmd/sqlterm
```

Para generar un binario local de desarrollo:

```sh
go build ./cmd/sqlterm
```

Estas alternativas son útiles durante el desarrollo, pero no son necesarias para iniciar normalmente una instalación ya realizada. SQLTerm necesita una terminal interactiva; se recomienda un tamaño de 120×35 o mayor. Los diálogos se adaptan al espacio disponible y no requieren una extensión de editor.

Go permite generar compilaciones para distintos sistemas y arquitecturas, pero no garantiza por sí solo toda la funcionalidad en tiempo de ejecución. Linux y Windows son los destinos principales; macOS se conserva como destino adicional de compilación. La compatibilidad práctica depende también del terminal, el portapapeles, el almacén de credenciales, las dependencias de la sesión, los controladores, la red y los servidores de bases de datos disponibles.

## Automatización local

La automatización portátil del repositorio se ejecuta con Go y no requiere PowerShell ni Bash:

```sh
go run ./scripts build
go run ./scripts check
go run ./scripts check -integration
```

`build` genera, en este orden, binarios para Windows, Linux y macOS, cada uno para `amd64` y `arm64`, con CGO desactivado. Los resultados se guardan en `dist/sqlterm-<sistema>-<arquitectura>`; solo los binarios de Windows usan el sufijo `.exe`.

`check` ejecuta `go test ./...` y después `go vet ./...`. La opción `-integration` añade las pruebas de `internal/db`; úsala únicamente con perfiles y bases desechables, nunca con datos reales o de producción.

Estas automatizaciones configuran cachés locales de Go bajo `.cache/build`, `.cache/go` y `.cache/mod` solo para sus procesos secundarios. Los comandos manuales de Go siguen disponibles para compilaciones y comprobaciones individuales.

## Primer flujo de trabajo

1. Pulsa **F2**, selecciona una carpeta y usa **+** o **Add profile**.
2. Completa motor, servidor, base y usuario. **Folder** indica la carpeta lógica del perfil.
3. Introduce la contraseña en el campo oculto. **Test connection** verifica los datos sin guardar el perfil ni la contraseña; durante la prueba cambia a **Cancel test** y la conexión temporal se cierra al terminar.
4. Guarda y selecciona el perfil. Si cambias motor, servidor, puerto efectivo o usuario de un perfil con credencial guardada, vuelve a introducir la contraseña antes de probar o guardar.
5. Pulsa **F6** para buscar un procedimiento, función, tabla o vista por nombre y filtrar por tipo.
6. **Enter** abre la definición del objeto. En rutinas, **F9** abre el formulario de parámetros y permite usar un valor, `NULL` o el valor predeterminado cuando el catálogo o la definición lo permite.
7. Ejecuta la rutina desde el formulario. `[` y `]` cambian entre conjuntos, incluidos los parámetros de salida y el retorno cuando el motor los ofrece.
8. Para editar una definición, pulsa **F7**, entra en INSERT con `i`, escribe y vuelve a NORMAL con **Esc**. **Espacio c A** muestra los cambios y el SQL exacto antes de aplicarlo.

**Espacio c t** genera una plantilla de procedimiento. Revísala y usa **Espacio c A** para aplicarla; una aplicación correcta actualiza el catálogo. Si guardas el archivo antes de aplicar, la comparación conserva como referencia la definición leída del servidor.

Para consultas libres, **Ctrl+N** crea otra pestaña con conexión independiente. **F5** ejecuta la selección o el documento completo. Solo se ejecuta una sentencia a la vez por pestaña, mientras que otras pestañas pueden trabajar en paralelo.

## Editor y archivos SQL

### Modos y selección

El panel SQL usa un proceso **Neovim real por pestaña**, iniciado con `--embed --clean -i NONE -n`. No carga ni modifica tu configuración personal de LazyVim y no conserva swap, shada ni historial de deshacer en disco. Neovim controla los modos, movimientos, registros y deshacer; SQLTerm conserva las conexiones, resultados y las acciones de archivos y ejecución.

Controles principales:

- `i a I A o`: insertar.
- **Esc**: volver a NORMAL.
- `v`: iniciar selección VISUAL por caracteres.
- `V` (mayúscula): seleccionar la línea lógica completa, incluido su salto de línea si existe; `j` y `k` extienden la selección por líneas.
- `gg`: ir al inicio del archivo; `G`: ir al final. `ggVG` selecciona todo el archivo.
- `hjkl`, `w b`, `0 $`: mover el cursor.
- `yy` y `dd`: copiar o cortar una línea.
- `y` y `d`: copiar o cortar la selección y volver a NORMAL.
- `x`: borrar; `p`: pegar; `u`: deshacer.
- **Ctrl+R** en NORMAL: rehacer.
- `/`: buscar con vuelta al inicio.

La selección sigue la semántica nativa de Neovim: `v` incluye el carácter actual y `V` selecciona líneas completas. `y` usa los registros de Neovim y vuelve a NORMAL; Ctrl+C copia al portapapeles del sistema sin cambiar el modo ni la selección. Usa Ctrl+S y Ctrl+Shift+S para guardar desde SQLTerm: el buffer de Neovim es temporal y no tiene una ruta propia.

En Windows Terminal, el backend actualizado usa pegado delimitado: Ctrl+V interceptado por la terminal llega como texto literal, no como comandos Vim, y no activa INSERT desde NORMAL. Cuando Ctrl+V llega como una tecla normal a Neovim, conserva su significado nativo de selección visual por bloques. La ejecución SQL no admite selecciones por bloques: muestra un error en lugar de ejecutar el documento completo.

Los atajos globales de SQLTerm (F1–F10, Ctrl+S, Ctrl+Shift+S, Ctrl+O, Ctrl+C, Ctrl+Q, Ctrl+N, Alt+Z) mantienen prioridad. Con el editor en modo NORMAL, Espacio abre el menú jerárquico; pulsa las letras del grupo y la acción, o Esc para cerrar. En INSERT, Espacio sigue escribiendo espacios. F11 y Ctrl+P ya no abren acciones; Ctrl+P conserva su función nativa y de navegación en IntelliSense. Los demás comandos del editor los interpreta Neovim. La sesión está aislada de tu configuración, pero Neovim no es un sandbox: sus comandos Ex siguen siendo comandos reales.

Si falta `nvim`, el inicio falla con un error; no se sustituye silenciosamente por el editor emulado. Si el hijo termina, se conserva el último texto confirmado y se bloquean guardado y ejecución desde datos obsoletos. Los errores al cargar texto no cambian la ruta ni el estado de guardado.

**Alt+Z** activa o desactiva el ajuste visual de línea sin modificar el contenido. La barra de estado conserva la línea y columna reales. La paleta permite mostrar u ocultar los números de línea. Estas preferencias se guardan y se aplican a todas las pestañas.

El resaltado distingue palabras reservadas, cadenas y números. Los delimitadores de identificadores no se colorean como cadenas. El dibujo respeta la selección y el ajuste de línea, y no modifica el SQL ni el historial de deshacer.

### Selección y ejecución

**F5** ejecuta el texto seleccionado; si no hay selección, ejecuta el documento. Las acciones de comentar o copiar no ejecutan SQL.

**Ctrl+}** o **Espacio c c** alterna comentarios en las líneas completas alcanzadas por la selección. Si la selección termina justo al inicio de otra línea, esa línea no se incluye. Sin selección, se comenta o descomenta la línea lógica completa donde está el cursor.

**Ctrl+Shift+}** o **Espacio c a** aplica la misma operación al documento. Se agrega `-- ` después de la indentación o se quita `--` y un espacio opcional. Las líneas mixtas se alternan por separado, se conservan la selección y los saltos de línea, y una acción completa se deshace como una sola operación.

También se acepta el código de control que algunas terminales envían como Ctrl+] (con Shift para todo el editor). Los atajos requieren que la terminal distinga Ctrl+} de Ctrl+Shift+}. Si no los comunica de forma diferenciada, usa las acciones de comentar de **Espacio c c**. Una `}` sin Ctrl sigue siendo texto normal.

Las líneas comentadas se muestran atenuadas, excepto marcadores dentro de cadenas o cuerpos con dollar quoting. El color es solo visual.

### Abrir, guardar y confirmar cambios

**Ctrl+O** abre el explorador integrado. **Enter** entra en carpetas o abre un archivo, `[..]` sube al nivel superior, Tab y Shift+Tab cambian de campo y **Esc** cancela. Se muestran todos los archivos para admitir scripts sin extensión `.sql`. La apertura admite archivos de hasta 8 MiB; si falla, conserva el documento actual.

**Ctrl+Shift+S** abre **Guardar SQL como / Save SQL as** cuando la terminal distingue la combinación. Algunas terminales pueden comunicarla como **Ctrl+S**; en ese caso se realiza el guardado normal. **Ctrl+S** guarda directamente cuando el archivo ya tiene ruta y abre el diálogo de guardado cuando todavía no la tiene.

El diálogo solicita confirmación antes de sobrescribir otro archivo. Cada guardado correcto recuerda la carpeta durante la sesión: el siguiente guardado como empieza allí y conserva el nombre de la pestaña, o propone `query.sql` para una pestaña nueva. La carpeta no se conserva entre sesiones. Si deja de existir, se usa la carpeta del archivo actual o el directorio de trabajo. El diálogo no crea carpetas.

Cerrar una pestaña, reemplazar SQL o salir solicita confirmación cuando existen cambios sin guardar. Las operaciones que generan una plantilla también confirman antes de reemplazar SQL modificado.

### Copiar SQL

Con foco en el editor, selecciona texto y pulsa **Ctrl+C** para copiarlo literalmente al portapapeles del sistema, conservando Unicode y saltos de línea. No añade cabeceras, formato TSV ni protección para hojas de cálculo; tampoco modifica el documento, la selección, el historial o el estado de guardado, ni ejecuta SQL.

Sin selección, se muestra un aviso y no se sobrescribe el portapapeles. Un carácter NUL se representa como `\0` solo en la copia y genera un aviso. Los errores se muestran en la barra de estado. La copia se procesa fuera del bucle de interfaz y no inicia otra copia SQL mientras la anterior sigue activa.

El acceso al portapapeles requiere un backend compatible con el sistema y la sesión gráfica o de terminal utilizada. En sesiones basadas en X11 puede requerirse `xclip` o `xsel`.

## Conexiones, perfiles y bases de datos

### Perfiles y prueba de conexión

Los perfiles incluyen motor, servidor, puerto, base, usuario, entorno, carpeta lógica y opciones de cifrado. El formulario muestra el puerto predeterminado del motor y lo actualiza al cambiar de motor si estaba vacío o conservaba el valor habitual; un puerto personalizado se mantiene. Un puerto `0` usa 1433, 3306 o 5432 según el motor.

**Test connection** no guarda cambios. La prueba puede cancelarse y siempre cierra su conexión temporal. Al editar nombre, carpeta o base, una contraseña vacía conserva la credencial guardada; al cambiar el endpoint efectivo se exige introducirla otra vez para impedir que se reutilice con otro destino.

### Base de datos opcional

Un perfil puede tener **Database** vacío. Al conectar, el explorador muestra las bases disponibles para ese usuario. **Enter** selecciona una y carga sus objetos; **F4** o **Espacio d b** permiten cambiarla después.

La selección pertenece a la pestaña y no modifica el perfil. Cada cambio abre una conexión nueva, mantiene el SQL y limpia resultados y metadatos de la base anterior. Si la nueva conexión falla, se mantiene la sesión previa. El estado transaccional no se conserva entre conexiones.

Hasta elegir una base no se permite ejecutar SQL, aplicar definiciones ni invocar rutinas. SQL Server usa `master` para enumerar bases, PostgreSQL necesita una conexión inicial a `postgres` y MySQL puede conectarse sin base. El usuario necesita acceso a la base inicial y permisos para enumerar las disponibles; si no los tiene, configura una base explícita.

### Carpetas y favoritos

**F2** organiza perfiles en carpetas lógicas, inicialmente DEV, QA y PROD. Admiten subcarpetas como `DEV/Ventas`; no son directorios del sistema. Los perfiles sin carpeta permanecen en la raíz.

Con foco en el árbol:

| Tecla | Acción |
| --- | --- |
| Enter | Conectar o expandir/contraer una carpeta (sin atajos con ←/→ ni Space) |
| + | Añadir un perfil en la carpeta seleccionada |
| n | Crear una carpeta o subcarpeta |
| e | Editar el perfil seleccionado |
| r | Renombrar una carpeta y sus descendientes |
| d | Eliminar una carpeta sin conexiones y sus subcarpetas vacías |
| / | Buscar por nombre, carpeta, motor, servidor o base |
| Tab / Shift+Tab | Recorrer búsqueda, árbol y botones |

Para mover un perfil, cambia **Folder**; dejarlo vacío lo devuelve a la raíz. Se permite el mismo nombre en distintas carpetas. Las credenciales conservan su referencia al mover o renombrar y la carpeta nunca cambia la base real de una sesión.

Pulsa `*` o **★ Favorito** para alternar un perfil favorito. Los favoritos se muestran en una sección superior, conservan su carpeta original y persisten entre sesiones.

## Catálogo, tablas y rutinas

El catálogo agrupa objetos por esquema y tipo. La búsqueda usa nombre, esquema y tipo y se conserva por pestaña. La paleta puede cambiar a lista plana, insertar el nombre con las comillas del motor, generar un `SELECT` para tablas o vistas e invocar una rutina. Un `SELECT` generado no se ejecuta automáticamente.

### Tablas

**Espacio c T** pide un nombre y prepara un `CREATE TABLE` sin ejecutarlo. Incluye una clave primaria `id` autogenerada y una columna `name`, adaptadas al motor. Usa `dbo` en SQL Server, `public` en PostgreSQL y la base seleccionada en MySQL. Requiere conexión y, para MySQL, una base activa.

Un clic o **Enter** sobre una tabla abre una definición `CREATE TABLE` para inspección. MySQL usa `SHOW CREATE TABLE`; SQL Server y PostgreSQL reconstruyen columnas, restricciones e índices desde los metadatos visibles.

Estas definiciones no son copias de seguridad ni migraciones completas: no incluyen permisos, triggers ni todas las opciones físicas. PostgreSQL no reconstruye tablas particionadas, heredadas o externas. SQL Server no reconstruye opciones especiales como tablas temporales del sistema, índices columnstore o particionado. **Aplicar definición** no modifica tablas; no ejecutes el `CREATE TABLE` sobre una tabla existente.

Los `SELECT` generados no añaden un límite fijo; usan la recepción incremental y respetan el esquema y las comillas del motor.

### Procedimientos y funciones

**F9** invoca el objeto seleccionado. Los valores se envían mediante el driver:

- SQL Server declara variables con longitud, precisión y escala del catálogo y devuelve salidas y `RETURN_STATUS` como conjunto adicional.
- PostgreSQL usa tipos explícitos para resolver sobrecargas y distingue `CALL` de funciones.
- MySQL usa variables de sesión para `OUT` e `INOUT` y las consulta al finalizar `CALL`.

**Espacio c A** revisa la diferencia y el SQL exacto antes de aplicar una rutina. SQL Server usa `ALTER` para objetos abiertos. PostgreSQL usa `CREATE OR REPLACE`; cambiar tipos de entrada puede crear otra sobrecarga y conservar la original. MySQL necesita `DROP` y `CREATE`: la revisión advierte que no es atómico y que quizá sea necesario restaurar permisos. Si falla `CREATE`, el editor conserva el código para corregirlo.

Actualizar el catálogo con **Espacio d r** permite encontrar objetos recién creados y limpia la caché de columnas cuando cambia el esquema.

## Ejecución y confirmaciones

### Comandos y estado

**F5** ejecuta SQL, **F9** invoca una rutina y **F10** cancela la operación activa. Los resultados identifican consulta, destino, número de ejecución, hora, duración y estado. Si editas el SQL después, se indica que los resultados pertenecen a una versión anterior.

La paleta ofrece **Execution confirmations**:

- Desactivadas, que es el valor predeterminado.
- Confirmación de escrituras probables en PROD.
- Confirmación en cada ejecución con F5.

La detección de escrituras es heurística: no garantiza seguridad, solo lectura ni identificación perfecta de efectos secundarios. Revisa siempre el SQL y los permisos de la cuenta.

### Recepción incremental

Cada ejecución obtiene primero `result_block_size` filas, 1000 por defecto, y queda **En pausa**. Una intención de desplazamiento al final de lo cargado —↓, Ctrl+N, PageDown, End, `j` o rueda hacia abajo— solicita el bloque siguiente. Redibujar, cambiar de pestaña, seleccionar otro conjunto o actualizar barras no solicita datos.

La pausa no bloquea la interfaz y el tiempo visible se congela. Al continuar, el tiempo se calcula desde el inicio original. Cada pestaña y ejecución mantiene su propia solicitud pendiente.

Una nueva ejecución, invocación, reconexión, cambio de base, desconexión, cierre de pestaña o salida cancela primero la lectura pausada y espera a que el driver cierre las filas y libere la conexión. Nunca hay dos sentencias simultáneas en la conexión de una misma pestaña.

Mientras se libera la lectura se muestra **Liberando la ejecución en pausa…**. La espera no tiene tiempo límite. Si el driver no responde, la pestaña permanece ocupada; **F10** puede descartar una operación nueva que estuviera esperando, pero no puede forzar al driver a liberar la lectura.

**F10** conserva las filas recibidas después de que el driver confirme la cancelación. Copiar, ordenar o exportar exige que la ejecución haya terminado; cancélala antes si está activa. Un conjunto cuyo tamaño sea múltiplo exacto del bloque requiere otro desplazamiento para comprobar el final. Los conjuntos y sentencias posteriores no avanzan hasta terminar el conjunto actual.

La consulta se ejecuta una sola vez: el cliente no añade `LIMIT`, `TOP` u `OFFSET` ni la reejecuta para cargar más filas. Pausar la lectura del cliente no garantiza que el servidor deje de procesar internamente.

## Resultados, copia y exportación

### Navegación e inspección

En resultados:

- **Enter** abre el valor completo de la celda.
- `/` busca.
- `s` y `S` ordenan localmente la columna en sentido ascendente o descendente.
- `m` muestra mensajes.
- `[` y `]` cambian de conjunto.
- `e` abre la exportación.

La ordenación es léxica, coloca `NULL` primero al ascender y solo actúa sobre las filas recibidas. La exportación conserva el orden original del servidor.

En SQL Server, los valores `uniqueidentifier` se muestran como GUID legibles, también en resultados de rutinas y bloques incrementales. Otras columnas binarias no se interpretan como GUID solo por tener 16 bytes. Una longitud binaria inválida produce un error de lectura en lugar de texto ilegible.

El inspector distingue `NULL` de `""`, muestra nombre y posición de columna y ofrece copia segura o literal. También copia nombre de columna y valor en una fila TSV.

### Copia segura y literal

| Tecla | Acción |
| --- | --- |
| Ctrl+C | Abrir el menú de copia |
| y / Y | Copiar todo sin / con cabeceras |
| r / R | Copiar la fila sin / con cabeceras |
| c | Copiar la celda |
| v | Iniciar o quitar una selección de filas |
| + / - | Ampliar o reducir la columna activa |

El menú permite copiar celda, celda con cabecera, fila o conjunto completo. La opción **Celda con cabecera** produce nombre, tabulador y valor en una sola fila TSV. Los tabuladores y saltos internos se entrecomillan para mantener la estructura.

El modo seguro es el predeterminado. Antepone `'` al texto cuyo primer carácter significativo sea `=`, `+`, `-` o `@`, tanto en cabeceras como en valores, para reducir la ejecución de fórmulas al pegar en una hoja de cálculo. También protege valores como `-42`. Usa **Literal** solo si necesitas el texto exacto y confías en los datos.

La transformación afecta únicamente a la salida. `NULL` se representa como `\N` y NUL como `\0`. Los fallos del portapapeles se muestran sin cerrar la aplicación. El menú se cierra con **Esc**, **Ctrl+C**, un clic fuera o un cambio de panel.

### Exportación y almacenamiento

`e` permite elegir explícitamente **Spreadsheet-safe CSV**, **Literal CSV** o **JSON**, con independencia del sufijo del archivo. La exportación exige un archivo nuevo, no sobrescribe uno existente y elimina la salida parcial de una exportación cancelada.

CSV seguro aplica la protección para hojas de cálculo a cabeceras y celdas. CSV literal conserva el texto y advierte sobre datos no confiables. Ambos usan `\N` para SQL NULL y escapan textos que comienzan con `\` para distinguir NULL, cadena vacía y el texto literal `\N`.

JSON usa `{"columns":[...],"rows":[[...]]}`, conserva nombres repetidos, representa SQL NULL como `null` y los demás valores como texto.

Cada conjunto se almacena en una carpeta temporal privada con un índice de offsets. La cuadrícula carga páginas de hasta 128 filas. Cambiar de consulta, cerrar una pestaña o salir normalmente elimina estos archivos; un cierre forzado puede dejar carpetas `sqlterm-results-*` en el directorio temporal del sistema. La ordenación mantiene en memoria solo índices y la exportación recorre el conjunto sin cargarlo completo.

## IntelliSense

Al iniciar, SQLTerm muestra su bienvenida en la barra de estado y mantiene el SQL vacío; no muestra la introducción de Neovim. Actualmente no restaura pestañas ni archivos de una ejecución anterior. Abre tus archivos con **Ctrl+O**.

En INSERT aparecen sugerencias después de escribir dos caracteres o un punto. **Ctrl+Space** las abre manualmente. Las flechas o **Ctrl+N/P** recorren la lista; **Tab** o **Enter** aceptan y **Esc** la cierra y vuelve a NORMAL.

Se sugieren palabras del dialecto, esquemas, tablas, vistas y rutinas con firma. Sin conexión solo hay palabras SQL. Los objetos y columnas requieren catálogo. SQLTerm reconoce referencias y alias simples en `FROM`, `JOIN`, `UPDATE` e `INTO`, y obtiene columnas cuando son necesarias.

En SQL Server, después de `EXEC procedimiento `, con un espacio final, sugiere parámetros nombrados. Tras una coma vuelve a ofrecerlos. Escribir `@` o un prefijo filtra y aceptar inserta `@parametro = `. La lista no se abre automáticamente en un valor vacío después de `=`. El procedimiento debe estar en el catálogo activo; actualízalo con **Espacio d r**.

Las sugerencias no ejecutan el SQL. No constituyen un servidor de lenguaje ni un analizador semántico completo: no infieren columnas de CTE o subconsultas complejas ni ofrecen diagnósticos equivalentes a una herramienta especializada.

## Interfaz, temas y carpetas

La barra superior muestra entorno, conexión, base y estado. PROD se destaca con texto y color. **Espacio w** cambia la distribución, ajusta proporciones, oculta el catálogo o maximiza paneles; la distribución se guarda. En terminales pequeñas se muestra un panel a la vez y **F6**, **F7** y **F8** cambian entre objetos, editor y resultados.

**F1** abre ayuda contextual con secciones y búsqueda. Los selectores de conexiones, temas y bases se cancelan con un clic fuera, **Esc** o **Ctrl+C**. Cambiar de panel los cierra. Cancelar el selector de temas revierte la vista previa.

### Temas

**F3** o **Espacio u t** abre el selector de temas. **Espacio c t** sigue reservado para la plantilla de procedimientos. Las flechas muestran una vista previa, **Enter** guarda y **Esc** restaura el tema anterior.

Se incluyen Tokyo Night, Rosé Pine, Nord, Gruvbox, Light y Classic. El tema cambia paneles, editor, resultados, formularios y diálogos sin desconectar ni reemplazar SQL. La elección persiste. El campo JSON `theme` acepta `tokyo-night`, `rose-pine`, `nord`, `gruvbox`, `light` o `classic`; un valor desconocido usa Tokyo Night.

La forma del cursor intenta cambiar de bloque en NORMAL y VISUAL a barra en INSERT, pero depende del soporte del terminal. El indicador de modo y el borde permanecen visibles. No se requieren iconos ni una fuente especial.

### Idioma

**Espacio u l** ofrece **Language / Idioma**. Los mensajes técnicos del motor, el SQL, los identificadores y los valores no se traducen.

## Atajos

| Tecla | Acción |
| --- | --- |
| F1 / F2 | Ayuda / conexiones |
| F3 / Espacio u t | Temas con vista previa |
| F4 | Seleccionar o cambiar base |
| F5 / F9 / F10 | Ejecutar / invocar / cancelar |
| F6 / F7 / F8 | Objetos / editor / resultados |
| Espacio (editor en NORMAL) | Menú jerárquico de acciones |
| Ctrl+Space | Abrir IntelliSense |
| Alt+Z | Alternar ajuste de línea |
| Ctrl+} | Comentar selección o línea actual |
| Ctrl+Shift+} | Comentar todo el editor |
| Ctrl+O / Ctrl+S / Ctrl+N | Abrir / guardar / nueva pestaña |
| Ctrl+Shift+S | Guardar como |
| Ctrl+C / Ctrl+Q | Copiar selección o abrir menú / salir |
| Alt+1…9 | Cambiar de pestaña |

### Menú de Espacio

Con el editor enfocado en NORMAL, pulsa **Espacio**, el grupo y la acción. Cada prefijo muestra las siguientes teclas disponibles. Las mayúsculas distinguen acciones. **Esc** cancela; **Retroceso** vuelve al grupo anterior. En INSERT, Espacio sigue escribiendo espacios. F11 y Ctrl+P no abren menús; Ctrl+P conserva la navegación de sugerencias.

| Grupo | Acciones después de Espacio |
| --- | --- |
| `f` Archivos | `o` abrir · `s` guardar · `S` guardar como |
| `b` Pestañas | `n` nueva sesión · `d` cerrar pestaña |
| `c` Código | `c` o `s` comentar selección o línea actual · `a` comentar documento · `x` ejecutar · `A` revisar/aplicar definición · `p` invocar rutina · `t` plantilla de rutina · `T` plantilla de tabla · `i` insertar nombre de objeto · `q` generar SELECT |
| `d` Conexiones | `c` perfiles · `b` cambiar base · `d` desconectar · `r` actualizar catálogo · `e` confirmaciones |
| `s` Explorar | `o` buscar objetos · `d` ver definición · `g` catálogo agrupado/plano · `h` historial |
| `r` Resultados | `y` copiar · `i` inspeccionar celda · `e` exportar · `b` tamaño de bloque · `m` mensajes técnicos |
| `u` Preferencias | `w` ajuste de línea · `n` números · `t` tema · `l` idioma |
| `w` Distribución | `b` equilibrada · `e` edición · `d` exploración · `m` maximizar editor · `r` maximizar resultados · `c` mostrar/ocultar catálogo · `>` / `<` ancho del catálogo ±4 columnas · `+` / `-` dar espacio al editor/resultados ±10 puntos porcentuales |
| Generales | `?` ayuda · `x` cancelar operación · `q q` salir |

Por ejemplo, **Espacio c c** alterna comentarios sin ejecutar ni guardar SQL. Si existe selección, actúa sobre las líneas alcanzadas; de lo contrario, sobre la línea completa del cursor. **Espacio w b** restaura la vista equilibrada tras maximizar.

`keys` permite alias globales con nombres de teclas especiales de tcell, por ejemplo `"run": "Ctrl-E"` o `"apply": "F12"`. Las acciones admitidas son `run`, `connect`, `invoke`, `cancel`, `help`, `save`, `open`, `new`, `apply`, `refresh`, `theme`, `database` y `complete`. F4 queda reservado para bases. F11 y Ctrl-P no se admiten como alias de acciones. Los alias tienen prioridad y los atajos predeterminados restantes continúan activos.

## Configuración y credenciales

La configuración predeterminada se encuentra en:

```text
os.UserConfigDir()/sqlterm/config.json
```

Para usar otro archivo con la instalación recomendada:

```sh
sqlterm --config examples/config.json
```

La ruta relativa `examples/config.json` funciona al ejecutar el comando desde la copia del repositorio. Desde otra carpeta, proporciona la ruta absoluta o una ruta relativa válida al archivo que quieras usar. El ejemplo contiene perfiles locales; los motores se identifican como `sqlserver`, `mysql` y `postgres`.

`result_block_size` acepta entre 1 y 100000 filas. Si falta o vale 0, se usan 1000. También puede cambiarse desde **Espacio r b**; solo se guarda cuando el valor y la escritura del archivo son válidos.

### Cifrado y certificados

El formulario contiene **Encrypt connection** y **Trust server certificate**. El cifrado está activado de forma predeterminada; confiar en el certificado es una decisión independiente. En JSON se guardan `encrypt` y `trust_server_certificate`.

Si aparece `x509: certificate signed by unknown authority`, el servidor presenta un certificado cuya autoridad no reconoce el sistema:

- Si verificaste que es el servidor esperado, puedes activar **Trust server certificate** manteniendo **Encrypt connection**. La conexión sigue cifrada, pero el certificado no se valida.
- Para validar sin esa opción, instala la autoridad certificadora correspondiente en el almacén de confianza del sistema.

SQLTerm nunca activa automáticamente la confianza. Edita el perfil, revisa el endpoint y vuelve a conectar. Confiar en un certificado sin validar reduce la protección frente a suplantación.

### Almacén de credenciales

Las contraseñas se guardan mediante el almacén de credenciales del sistema. El JSON contiene un `credential_id`, no la contraseña. Eliminar un perfil elimina también su credencial. Si el almacén no está disponible, la aplicación informa el error y no guarda la contraseña en texto plano.

La disponibilidad y configuración del almacén dependen del sistema y de la sesión; algunos entornos requieren un servicio compatible como Secret Service. Los perfiles antiguos continúan siendo compatibles y `password_env` se admite en archivos existentes.

Los errores de conexión ocultan la contraseña conocida, incluso cuando aparece codificada dentro de una URL o DSN. Cambiar motor, servidor, puerto efectivo o usuario invalida la asociación anterior y exige volver a introducir la contraseña.

## Registro de errores e historial

### Registro de errores

Los errores técnicos completos se anexan a `sqlterm-errors.log`, junto al archivo de configuración seleccionado, incluso al usar `--config`. El archivo se crea solo cuando ocurre un error y SQLTerm no lo rota. El usuario debe eliminarlo manualmente cuando ya no sea necesario.

SQLTerm redacta contraseñas conocidas y sus formas codificadas. No agrega por separado SQL, resultados, configuración o DSN, pero el texto devuelto por un servidor puede contener fragmentos de SQL o datos. Revisa el archivo antes de compartirlo y elimínalo si contiene información sensible.

### Historial opcional

El historial está desactivado por defecto. **Espacio s h** permite activarlo; se guarda como `history.json` junto a los datos del perfil y conserva como máximo 200 consultas.

El historial puede incluir literales sensibles escritos en SQL. Los valores del formulario de invocación se excluyen. Las actualizaciones reemplazan el archivo de forma atómica y se coordinan entre procesos que comparten la misma ruta. Desactivar el historial conserva lo existente; **Clear history** lo elimina. El contenido del editor no se guarda automáticamente al salir.

## Scripts y comportamiento por motor

- SQL Server conserva el lote y reconoce `GO` y `GO n`, hasta 1000, en líneas independientes fuera de cadenas y comentarios.
- MySQL reconoce `DELIMITER` al crear rutinas. Aplicar una definición envía el cuerpo completo sin necesitar esa directiva.
- PostgreSQL reconoce cuerpos dollar-quoted, comentarios anidados y cadenas `E''`.
- El separador no se implementa con un simple `strings.Split(";")`.

Las transacciones siguen las reglas del servidor. Usa `BEGIN`, `COMMIT` y `ROLLBACK` explícitamente. Cambiar de base abre otra conexión y pierde el estado temporal de la anterior. No existen reintentos automáticos de escrituras.

Los parámetros del formulario son texto o `NULL` y el servidor realiza la conversión. Los parámetros tabulares de SQL Server y tipos compuestos que requieren carga especial deben invocarse mediante SQL. Los arrays de PostgreSQL se introducen en representación textual y los cursores devueltos requieren `FETCH` explícito.

## Pruebas

Las pruebas de Neovim usan procesos aislados y se omiten si `nvim` no está instalado; esa omisión no acredita la integración real. Con Neovim instalado, `go test ./internal/nvim ./internal/editor ./internal/ui -count=1` cubre el motor y su integración.

Para comprobar el transporte nativo en Windows Terminal, ejecuta `./scripts/verify-windows-terminal.ps1` desde PowerShell y sigue sus indicaciones de pegado, teclas, mouse y redimensionado. Es una comprobación interactiva opcional; las pruebas automáticas del parser no sustituyen esa evidencia ni prueban la restauración real de consola.

Para las comprobaciones habituales:

```sh
go test ./...
go vet ./...
```

Si el entorno y las dependencias lo permiten, también puede usarse:

```sh
go test -race ./...
```

Las pruebas de integración se omiten salvo que se configuren bases desechables. Crean y eliminan procedimientos y tablas con nombres únicos: **no uses servidores ni bases de producción**.

Cada motor se activa con variables de entorno independientes:

- `SQLTERM_TEST_POSTGRES_PROFILE` y `SQLTERM_TEST_POSTGRES_PASSWORD`.
- `SQLTERM_TEST_SQLSERVER_PROFILE` y `SQLTERM_TEST_SQLSERVER_PASSWORD`.
- `SQLTERM_TEST_MYSQL_PROFILE` y `SQLTERM_TEST_MYSQL_PASSWORD`.

Configura la variable `*_PROFILE` con un objeto JSON que incluya el host, puerto, base desechable, usuario y opciones de conexión que correspondan. Configura `*_PASSWORD` por separado. Usa los mecanismos de variables de entorno de tu terminal y evita persistir credenciales en el repositorio.

La automatización del repositorio ejecuta pruebas de Go en varios entornos y proporciona servicios desechables para los tres motores. Esto verifica la orientación de compilación y el flujo automatizado; no demuestra por sí solo la compatibilidad real con cada terminal, almacén de credenciales, dependencia nativa, red o servidor existente.

## Límites conocidos

- El alcance se orienta a SQL Server, MySQL 8.0+ y PostgreSQL 14+.
- La integración automatizada usa versiones concretas; otras versiones necesitan validación real.
- La autenticación disponible es usuario y contraseña.
- El usuario conectado solo puede ver y operar objetos permitidos por sus privilegios. Las definiciones cifradas o sin permiso se muestran como inaccesibles.
- El editor ofrece modos Vim básicos, no una implementación completa de Neovim.
- IntelliSense depende del catálogo y no realiza análisis semántico avanzado ni búsqueda dentro de todas las definiciones.
- La compatibilidad SQL depende del dialecto y de los modos del servidor. El lexer no se adapta a `NO_BACKSLASH_ESCAPES` ni a `standard_conforming_strings=off`.
- Los cuerpos de rutinas PostgreSQL deben usar dollar quoting o cadenas. Los cuerpos SQL `BEGIN ATOMIC` sin comillas se aplican como definición completa con **Espacio c A**, no como documento separado por F5.
- Las vistas materializadas PostgreSQL no tienen flujo de edición dedicado.
- Las instancias de SQL Server accesibles por nombre deben configurarse con su host y puerto TCP efectivos.
- La compatibilidad completa con versiones anteriores de SQL Server requiere validación real, aunque la construcción de firmas no depende de `STRING_AGG`.
- La edición de datos en cuadrícula, diseñadores, comparación de esquemas, profiler, backup y restore no forman parte del alcance actual.
- La disponibilidad del portapapeles, el cursor, los colores, el almacén de credenciales y otros servicios depende del entorno.

## Estructura del repositorio

- `cmd/sqlterm`: entrada y flags.
- `internal/db`: adaptadores, metadatos, scripts, invocación y eventos de ejecución.
- `internal/editor`: controles Vim sobre un buffer tview.
- `internal/ui`: paneles, formularios y coordinación de sesiones.
- `internal/result`: almacenamiento paginado y exportación.
- `internal/config` e `internal/history`: persistencia local.
- `examples`: archivos de configuración de ejemplo.
- `scripts`: automatización de comprobaciones y compilación.
