# SQLTerm

Cliente de terminal en Go para **SQL Server, MySQL y PostgreSQL**, con editor
integrado de controles Vim. Diseñado alrededor de buscar procedimientos,
consultar su código, modificarlos y ejecutarlos con parámetros.

## Ejecutar

Requiere Go 1.23 o posterior para compilar. El binario no necesita Go ni Neovim.

```powershell
cd C:\Users\dcaceresc\source\personal\sqlterm
go run ./cmd/sqlterm
```

En este espacio de trabajo, los scripts usan cachés locales para evitar los
permisos restringidos de las carpetas globales de Go:

```powershell
.\scripts\check.ps1
.\scripts\build.ps1
.\dist\sqlterm-windows-amd64.exe
```

El script de compilación genera Windows, Linux y macOS, para amd64 y arm64, en
`dist/`. En Unix también puedes usar `go build -o sqlterm ./cmd/sqlterm`.

Si la política de PowerShell no permite scripts, puedes compilar directamente
sin modificarla:

```powershell
$env:GOCACHE = "$PWD/.cache/build"
$env:GOPATH = "$PWD/.cache/go"
$env:GOMODCACHE = "$PWD/.cache/mod"
go build -o dist/sqlterm.exe ./cmd/sqlterm
.\dist\sqlterm.exe
```

Abre la aplicación en una terminal interactiva; se recomienda 120×35 o mayor.
Los diálogos se ajustan al tamaño disponible. Activa teclado y ratón de tu
terminal habitual; no se requiere una extensión del editor.

## Primer flujo

1. Pulsa **F2**, selecciona una carpeta y pulsa **+** o **Add profile**. Completa
   el motor, servidor, base y usuario; el campo **Folder** indica su carpeta.
2. Introduce la contraseña en el campo oculto, guarda el perfil y selecciónalo.
   **Test connection** permite verificar los datos antes de guardar; durante la
   prueba se convierte en **Cancel test**. No guarda el perfil ni la contraseña,
   y cierra la conexión temporal al terminar.
3. Pulsa **F6** y busca el procedimiento por nombre. Puedes filtrar por tipo.
4. **Enter** abre su definición. **F9** abre sus parámetros: valor, `NULL` o
   predeterminado cuando el catálogo/definición lo permite.
5. Ejecuta desde el formulario. Los resultados aparecen debajo; `[` y `]`
   cambian entre conjuntos, incluidos parámetros de salida y retorno.
6. Para editar, pulsa **F7**, `i` y escribe; vuelve con **Esc**. **Space a**
   muestra los cambios y el SQL exacto antes de aplicar al servidor.

**Space t** genera una plantilla de procedimiento. Usa **Space a** para aplicarla;
después búscala con F6 para abrirla como objeto del catálogo. Cada aplicación
correcta actualiza el catálogo. Si guardas el archivo antes de aplicarlo, la
comparación conserva como referencia la definición leída del servidor.

Para consultas libres, **Ctrl+N** crea otra pestaña con una conexión independiente.
**F5** ejecuta la selección o todo el documento. **F10** cancela. Una pestaña
admite una operación a la vez; las demás pueden ejecutar en paralelo.
**Alt+1…9** cambia de pestaña.

## Capacidades

| Área | Implementado |
| --- | --- |
| Conexiones | Perfiles, contraseña guardada, cifrado, confianza del certificado, cambio de base mediante reconexión |
| Catálogo | Tablas, vistas, procedimientos, funciones, búsqueda aproximada por nombre, filtro por tipo |
| Rutinas | Definición, edición, creación, revisión, invocación parametrizada, salida/retorno según motor |
| Editor | Pestañas, archivos `.sql`, modos Vim básicos, deshacer/rehacer, búsqueda, aviso de cambios sin guardar |
| Asistencia | Palabras del dialecto, objetos, columnas de tablas mencionadas, firmas de rutinas en sugerencias |
| Ejecución | Selección/documento, lotes, cancelación, errores, duración, conjuntos múltiples, mensajes y filas afectadas |
| Resultados | Cuadrícula virtual, `NULL` visible, visor de celdas, búsqueda, orden lexical local, CSV/JSON |
| Historial | Opt-in, máximo 200 consultas, recuperación, borrado y desactivación |

Los parámetros se envían mediante el driver. SQL Server declara variables con
longitud, precisión y escala del catálogo; devuelve las salidas y `RETURN_STATUS`
como un conjunto adicional. PostgreSQL usa tipos explícitos para resolver
sobrecargas y distingue `CALL` de funciones. MySQL usa variables de la sesión
para `OUT`/`INOUT` y las consulta al terminar `CALL`.

## Atajos

| Tecla | Acción |
| --- | --- |
| F1 / F2 | Ayuda / conexiones |
| F3 / Space T | Selector de temas con vista previa |
| F4 | Seleccionar o cambiar base de datos |
| F5 / F9 / F10 | Ejecutar SQL / invocar objeto / cancelar |
| F6 / F7 / F8 | Buscar objetos / editor / resultados |
| F11 | Distribución: editar, explorar, equilibrado, maximizar y ajustar tamaños |
| Ctrl+P / Space en NORMAL | Paleta de acciones; `/` busca y Tab cambia al buscador |
| Ctrl+Space | Abrir IntelliSense; también aparece automáticamente al escribir |
| Ctrl+O / Ctrl+S / Ctrl+N | Abrir archivo / guardar / nueva sesión |
| Ctrl+C / Ctrl+Q | Menú de copia desde cualquier panel / salir |
| Space c/f/x/p/a/r | Conectar / buscar / ejecutar / invocar / aplicar / actualizar catálogo |
| Space n/w/o/q | Nueva pestaña / guardar / abrir / cerrar pestaña |
| Space h/e/b/d/t | Historial / exportar / cambiar base / desconectar / plantilla |

Los atajos con **Space** se usan en modo **NORMAL** y con foco en el editor.
Space abre la paleta y la segunda tecla conserva la acción existente (por ejemplo,
Space c conecta). Ctrl+P está disponible desde cualquier panel.

## Interfaz y contexto de ejecución

La barra superior muestra entorno, conexión, base y estado de la sesión. PROD
se destaca mediante texto y color. Los resultados identifican consulta, destino,+número de ejecución, hora, duración y estado. Editar el SQL marca los resultados
como pertenecientes al SQL anterior. F10 muestra **Cancelando** hasta que el
controlador confirme la finalización; las filas parciales quedan identificadas.

El catálogo agrupa por esquema y tipo; Enter expande grupos o abre definiciones.
La búsqueda incluye nombre, esquema y tipo y se conserva por pestaña. En la
paleta puedes cambiar a lista plana, insertar el nombre entre comillas, generar
un SELECT para tablas/vistas o invocar una rutina. Estas acciones no ejecutan
automáticamente un SELECT generado.

F1 abre ayuda contextual, con secciones y búsqueda. F2 incluye búsqueda inicial,
detalles del perfil, recientes y favoritos (`*` sobre el perfil). En F4 se marca
la base actual y se explica el reemplazo de sesión: conserva el SQL y mantiene
la sesión anterior si falla la nueva conexión; no conserva estado transaccional
entre conexiones. Esc devuelve el foco al control que abrió el diálogo.

F11 permite ajustar la proporción editor/resultados, el ancho del catálogo,
ocultarlo y maximizar paneles. La distribución se guarda. En 80×24 se muestra un
panel a la vez; F6/F7/F8 alternan entre ellos y el destino permanece visible.

La paleta también ofrece **Language / Idioma** (English / Español) y
**Execution confirmations**: desactivadas por defecto, escrituras probables en
PROD o cada ejecución con F5. La detección de escrituras es heurística y no
garantiza seguridad ni solo lectura. Los mensajes técnicos del motor conservan
su idioma original. SQL, nombres y valores nunca se traducen.

La revisión y las decisiones de implementación están en
[Propuesta-UI/IMPLEMENTACION.md](Propuesta-UI/IMPLEMENTACION.md).

Editor: `i a I A o` para insertar; `Esc` normal; `v` visual; `hjkl`, `w b`,
`0 $`, `gg G` para mover; `yy/dd` copiar/cortar línea; `y/d` sobre selección;
`x` borrar; `p` pegar; `u` deshacer; `Ctrl+R` rehacer; `/` buscar con vuelta.
La selección visual usa un intervalo semiabierto: `vl` selecciona un carácter.
El portapapeles es interno al editor. No implementa macros, registros ni Ex.

La barra inferior sigue el esquema visual de una statusline de LazyVim: un bloque
de modo con fondo sólido, una sección con archivo y conexión y la posición SQL
alineada a la derecha. **NORMAL**, **INSERT** y **VISUAL** tienen colores distintos
en cada tema. INSERT usa verde o cian; VISUAL usa un tono violeta, rosa o naranja
según la paleta. El borde y el título del editor activo acompañan ese color.

El cursor cambia de **bloque** en NORMAL/VISUAL a **barra vertical** en INSERT.
Al entrar en un formulario recupera el cursor habitual del terminal. La forma y
el color del cursor requieren soporte del terminal; el indicador de modo y el
borde siempre se muestran. No requiere una Nerd Font ni iconos especiales.

Resultados: `Enter` muestra la celda completa; `/` busca; `s/S` ordena la columna
seleccionada ascendente/descendente; `e` exporta; `m` muestra mensajes;
`[`/`]` cambia de conjunto. La ordenación es lexical y coloca `NULL` primero
en ascendente; la exportación conserva el orden original del servidor.

Para copiar resultados al portapapeles:

| Tecla en resultados | Acción |
| --- | --- |
| Ctrl+C | Menú para copiar celda, fila o todo el conjunto, con o sin cabeceras |
| y / Y | Todo el conjunto sin / con cabeceras |
| r / R | Fila actual sin / con cabeceras |
| c | Celda actual |
| v | Iniciar / quitar selección de un rango de filas; mueve el cursor y usa Ctrl+C |
| + / - | Ampliar / reducir el ancho de la columna activa |

Enter abre el inspector con el valor completo y un botón de copia. La línea de
detalle muestra el nombre completo de la columna y su posición. `NULL` indica
SQL NULL y `""` una cadena vacía. El orden con s/S es léxico y local a las filas
recibidas. El menú de copia indica el origen, alcance y cantidad de filas.

El menú de copia se cierra con Esc, Ctrl+C o un clic fuera de él. Al cambiar de
panel con F6/F7/F8 también se cierra. Ctrl+C permite abrirlo de nuevo desde el
editor, objetos o resultados; Ctrl+Q sale de la aplicación.

Los selectores de conexiones (F2), temas (F3) y bases (F4) siguen el mismo
comportamiento: clic fuera, Esc o Ctrl+C cancelan; F6/F7/F8 cierran el selector
y cambian de panel. F2/F3/F4 permiten abrir otro selector. Cancelar el selector
de temas revierte la vista previa.

Las filas se copian como TSV, en el orden visible de la cuadrícula, conservando
tabulaciones y saltos de línea mediante comillas. SQL NULL usa `\N`, igual que CSV.
Los caracteres NUL se representan como `\0` al copiar, sin modificar los resultados.
Los fallos del portapapeles se muestran en la barra de estado y mantienen abierta la aplicación.
La copia espera a que termine la consulta. En Linux se requiere un proveedor de
portapapeles compatible disponible en la sesión, como `xclip` o `xsel`.

## Base de datos opcional

Puedes guardar un perfil con **Database** vacío. Al conectar, el explorador muestra
las bases disponibles para ese usuario. **Enter** selecciona una base y carga sus
objetos. **F4** abre el selector de bases; también permite cambiar una base ya
seleccionada. **Space b** conserva la misma acción. El filtro de tipos queda
desactivado mientras se muestran bases en lugar de objetos.

La selección se aplica a esa pestaña y no modifica el perfil guardado. Cada cambio
abre una conexión nueva; el SQL del editor permanece, y los resultados y metadatos
de la base anterior se limpian. Hasta seleccionar una base no se permite ejecutar
SQL, aplicar definiciones ni invocar rutinas en la base de entrada del servidor.

SQL Server usa `master` para enumerar bases; PostgreSQL necesita una conexión
inicial a `postgres`; MySQL puede conectarse sin base. El usuario necesita acceso
a esa base inicial y permisos para ver las bases disponibles. Si no tiene ese
acceso, indica una base explícita en el perfil.

## IntelliSense

En INSERT aparecen sugerencias al escribir dos caracteres o un punto. **Ctrl+Space**
las abre manualmente. Las flechas o **Ctrl+N/P** recorren opciones; **Tab** o **Enter**
aceptan; **Esc** cierra la lista y vuelve a NORMAL. La lista flota junto al cursor y
mantiene el foco en el editor.

Sugiere palabras del dialecto, esquemas, tablas, vistas y rutinas con su firma.
Sin conexión ofrece palabras SQL; los objetos y sus columnas requieren catálogo.
Obtiene columnas del catálogo al necesitarlas y reconoce referencias y alias simples
de `FROM`, `JOIN`, `UPDATE` e `INTO`, por ejemplo `SELECT c.Na` con
`FROM dbo.Customers c`. Para SQL Server también sugiere parámetros nombrados tras
`EXEC procedimiento @pa`. No ejecuta el SQL escrito para generar sugerencias.

No es un servidor de lenguaje ni un analizador semántico completo: no infiere
columnas de CTE o subconsultas complejas, ni muestra diagnósticos equivalentes al
servicio de lenguaje de Microsoft. **Space r** actualiza el catálogo y limpia
la caché de columnas si cambió el esquema.

## Temas

**F3** abre el selector desde el editor o el diálogo de conexiones. También puedes
usar **Space T** (T mayúscula) en modo NORMAL; **Space t** mantiene la plantilla de
procedimientos. Las flechas muestran una vista previa, **Enter** guarda la elección
y **Esc** restaura el tema anterior.

Incluye **Tokyo Night**, **Rosé Pine**, **Nord**, **Gruvbox**, **Light** y **Classic**.
Las paletas cambian paneles, editor, resultados, selección, formularios y diálogos
sin desconectar sesiones ni reemplazar el SQL abierto. La elección se conserva al
reiniciar. El tema predeterminado es Tokyo Night; un nombre desconocido utiliza
ese mismo tema.

El campo JSON `theme` acepta `tokyo-night`, `rose-pine`, `nord`, `gruvbox`, `light`
o `classic`. Se puede asignar un alias de teclado con `"theme": "F11"` dentro de
`keys`. Las opciones visuales toman como referencia los temas anunciados por
[sqlit](https://github.com/Maxteabag/sqlit); están implementadas directamente en Go.

## Carpetas de conexiones

**F2** muestra un árbol con DEV, QA y PROD como carpetas iniciales. Las conexiones
anteriores sin carpeta permanecen en la raíz. Son carpetas lógicas de la aplicación,
no directorios del disco, y admiten subcarpetas como `DEV/Ventas`.

Con foco en el árbol:

| Tecla | Acción |
| --- | --- |
| Enter | Conectar a un perfil o expandir/contraer una carpeta |
| + | Añadir un perfil dentro de la carpeta seleccionada |
| n | Crear una carpeta o subcarpeta |
| e | Editar el perfil seleccionado |
| r | Renombrar una carpeta y sus descendientes |
| d | Eliminar una carpeta sin conexiones, incluidas sus subcarpetas vacías |
| / | Buscar por nombre, carpeta, motor, servidor o base |
| Tab / Shift+Tab | Recorrer búsqueda, árbol y botones |

Para mover una conexión, edítala y cambia **Folder**; dejarlo vacío la devuelve a
la raíz. Puedes usar el mismo nombre de conexión en diferentes carpetas. Las
contraseñas conservan su referencia en el almacén del sistema al mover o renombrar.
La barra de estado y el historial muestran `carpeta/conexión` para distinguir los
entornos. La carpeta nunca cambia la base real de una sesión.

## Configuración

La ruta predeterminada usa `os.UserConfigDir()/sqlterm/config.json`:
`%AppData%\sqlterm\config.json` en Windows, `$XDG_CONFIG_HOME/sqlterm/config.json`
o `~/.config/sqlterm/config.json` en Linux, y
`~/Library/Application Support/sqlterm/config.json` en macOS.

```powershell
.\dist\sqlterm-windows-amd64.exe --config .\examples\config.json
```

`examples/config.json` contiene perfiles locales. Los nombres de motor son
`sqlserver`, `mysql` y `postgres`; un puerto `0` usa 1433, 3306 o 5432.
El formulario muestra directamente el puerto predeterminado del motor. Al cambiar
de motor lo actualiza si estaba vacío o tenía el valor habitual; conserva un puerto
personalizado. Guardar el campo vacío también selecciona el valor predeterminado.
El formulario contiene **Password**, **Encrypt connection** y **Trust server
certificate**. Ya no contiene selectores TLS, archivos CA ni variables de entorno.
El cifrado está activado por defecto; confiar en el certificado es una opción
independiente. En JSON se guardan `encrypt` y `trust_server_certificate`.

La contraseña se guarda automáticamente en el Administrador de credenciales de
Windows, Keychain de macOS o Secret Service de Linux. El campo se muestra con
asteriscos. Al editar, dejarlo vacío conserva la contraseña guardada; introducir
otra la reemplaza. El JSON contiene únicamente un `credential_id`, nunca la
contraseña. Al eliminar el perfil también se elimina su credencial.

Linux requiere una sesión de Secret Service disponible, por ejemplo GNOME Keyring
o un proveedor compatible. Si el almacén no está disponible, la aplicación muestra
el error y no guarda contraseñas en texto plano. Los perfiles anteriores siguen
siendo compatibles; `password_env` aún se admite en archivos existentes.

`keys` permite alias globales con nombres de teclas especiales de tcell, por
ejemplo `"run": "Ctrl-E"` o `"apply": "F12"`. Acciones: `run`, `connect`, `invoke`,
`cancel`, `help`, `save`, `open`, `new`, `apply`, `refresh`, `theme`, `database`,
`complete`. F4 queda reservado para seleccionar bases. Acepta también `+`
en lugar de `-`. Los alias tienen prioridad sobre los atajos predeterminados;
los demás atajos predeterminados siguen activos.

El historial está desactivado por defecto; **Space h** permite activarlo.
Se guarda junto al perfil, en `history.json`. Puede contener literales sensibles
escritos en SQL. Los valores del formulario de invocación se excluyen del
historial. Desactivarlo conserva los registros anteriores; **Clear history**
los elimina. No se guarda el contenido del editor automáticamente al salir.

## Scripts y cambios en el servidor

### Error de certificado TLS

Si aparece `x509: certificate signed by unknown authority`, el servidor presenta
un certificado cuya autoridad no conoce tu equipo. El diálogo del error permite
abrir **Edit profile**:

- Si este es el servidor esperado, activa **Trust server certificate** y conserva
  **Encrypt connection** activado. La conexión permanece cifrada y se acepta su
  certificado sin validarlo.
- Para validar el certificado sin marcar esa opción, su CA debe estar en el
  almacén de confianza del sistema.

Guarda y selecciona nuevamente la conexión. También puedes editar el perfil desde
**F2 → Edit profile**. La aplicación no activa automáticamente la confianza en el
certificado.

### Ejecución de scripts

- SQL Server conserva el lote completo y reconoce `GO`, también `GO n` hasta
  1000, en líneas independientes fuera de comentarios/cadenas.
- MySQL reconoce `DELIMITER` para scripts que crean rutinas. El botón de aplicar
  definición envía el cuerpo completo sin necesitar esta directiva.
- PostgreSQL reconoce cuerpos dollar-quoted, comentarios anidados y cadenas `E''`.
- No se divide SQL usando simplemente `strings.Split(";")`.
- Las transacciones siguen las reglas del servidor. Usa `BEGIN`, `COMMIT` y
  `ROLLBACK` explícitos; cambiar de base abre otra conexión y pierde el estado
  temporal de la anterior. No hay reintentos automáticos de escrituras.
- SQL Server aplica `ALTER` a objetos abiertos; PostgreSQL usa `CREATE OR REPLACE`.
  Cambiar los tipos de entrada de una rutina PostgreSQL crea otra sobrecarga;
  el original permanece y la revisión lo indica.
  MySQL necesita `DROP` y `CREATE` para modificar rutinas: la revisión lo muestra
  y advierte que no es atómico y que pueden necesitar restaurarse los permisos.
  Si falla `CREATE`, el editor conserva el código para corregirlo.

## Almacenamiento y exportación

Los resultados se almacenan en una carpeta temporal privada por conjunto, con
índice de offsets en disco. La cuadrícula solo carga páginas de hasta 128 filas.
Cambiar de consulta, cerrar pestaña o salir normalmente elimina esos archivos.
Un cierre forzado del proceso puede dejar carpetas `sqlterm-results-*` en el
directorio temporal del sistema.

La ordenación conserva en memoria únicamente índices de filas. CSV/JSON recorren
el conjunto completo sin cargarlo entero. La exportación exige un archivo nuevo;
no sobrescribe archivos existentes y elimina exportaciones parciales canceladas.

CSV representa `NULL` como `\N` y antepone otra barra a textos que empiezan por
`\`, de modo que `NULL`, cadena vacía y el texto literal `\N` son distinguibles.
JSON usa `{"columns":[...],"rows":[[...]]}`, conserva nombres de columna repetidos,
representa SQL NULL como `null` y los valores no nulos como texto.

## Pruebas

```powershell
.\scripts\check.ps1
```

O, en un entorno con caché de Go disponible:

```sh
go test ./...
go vet ./...
go test -race ./...
```

Las pruebas de integración se omiten salvo que se configuren bases desechables.
Crean y eliminan procedimientos con nombres únicos; **no uses producción**.
Cada motor se activa de forma independiente:

```powershell
$env:SQLTERM_TEST_POSTGRES_PROFILE = '{"host":"localhost","port":5432,"database":"sqlterm_test","user":"postgres","tls":"disable"}'
$env:SQLTERM_TEST_POSTGRES_PASSWORD = 'test_password'
.\scripts\check.ps1 -Integration
```

Para los otros motores usa `SQLTERM_TEST_SQLSERVER_PROFILE` y
`SQLTERM_TEST_SQLSERVER_PASSWORD`, o `SQLTERM_TEST_MYSQL_PROFILE` y
`SQLTERM_TEST_MYSQL_PASSWORD`, con el mismo formato. El workflow de GitHub Actions
define servicios desechables de los tres motores y pruebas de Go en los tres
sistemas. Añadir el workflow no lo publica ni lo ejecuta remotamente.

## Límites de esta versión

Se orienta a SQL Server, MySQL 8.0+ y PostgreSQL 14+. El catálogo SQL Server no
depende de `STRING_AGG`: las firmas se construyen en Go para admitir servidores
anteriores a SQL Server 2017. La integración automatizada usa SQL Server 2022;
la compatibilidad completa con versiones anteriores necesita validación real.
La autenticación es
usuario/contraseña. El editor no equivale a Neovim completo y las sugerencias
proceden del catálogo, sin diagnóstico semántico ni resolución avanzada de alias.
No busca dentro del contenido de todas las definiciones.

SQL Server se conecta por host y puerto TCP; para instancias con nombre,
configura su puerto efectivo. El estado de las verificaciones realizadas y
pendientes está en [VALIDATION.md](VALIDATION.md).

Los parámetros tabulares de SQL Server y tipos compuestos que requieren carga
especial deben invocarse mediante un script SQL. Los valores del formulario son
texto o NULL y el servidor realiza su conversión; arrays PostgreSQL se introducen
en su representación textual. Los cursores devueltos por PostgreSQL requieren
`FETCH` explícito. Las definiciones cifradas o no visibles por permisos se informan
como inaccesibles. Solo se muestran objetos que permite ver el usuario conectado.

El separador asume los modos habituales de cadenas del motor; no adapta el lexer
a `NO_BACKSLASH_ESCAPES` de MySQL ni a `standard_conforming_strings=off` de
PostgreSQL. Los cuerpos de rutinas PostgreSQL deben usar dollar-quoting o cadenas;
los cuerpos SQL `BEGIN ATOMIC` sin comillas se ejecutan como definición completa
con **Space a**, no como documento separado por F5.

Las vistas materializadas PostgreSQL no tienen un flujo de edición dedicado.
La edición de datos en cuadrícula, diseñadores, comparación de esquemas, profiler,
backup/restore e integraciones Azure/Copilot quedan para versiones posteriores.

## Estructura

- `cmd/sqlterm`: entrada y flags.
- `internal/db`: adaptadores, metadatos, scripts, invocación y eventos de ejecución.
- `internal/editor`: controles Vim sobre un buffer tview.
- `internal/ui`: paneles, formularios y coordinación de sesiones.
- `internal/result`: almacenamiento paginado y exportación.
- `internal/config` y `internal/history`: persistencia local.
