# Issues

## Resumen de estado

Estados contrastados con el código actual y sus pruebas automatizadas. La suite
`go test ./... -count=1` y `go vet ./...` finalizaron correctamente. Las issues
7, 9 y 10 también cuentan con confirmación funcional del usuario.

| Issue | Estado | Resultado o pendiente |
| --- | --- | --- |
| 4 | Solucionada | Carga incremental configurable por bloques bajo demanda. |
| 5 | Cerrada: no se hará | Se conserva Ctrl+W; falta mencionarlo en la ayuda. |
| 6 | Solucionada | Copiar celda con cabecera, con variantes segura y literal. |
| 7 | Solucionada | Los uniqueidentifier se muestran como GUID legibles. Commit `6adc2c1`. |
| 8 | Solucionada | Ctrl+C copia la selección SQL al portapapeles; sin selección muestra un aviso y no copia. |
| 9 | Solucionada | Comentar/descomentar selección o documento y color atenuado. Commit `5f235db`. |
| 10 | Solucionada | Explorador integrado para abrir y Guardar como. Commit `76ff3c9`. |
| 11 | Parcial | Existe F11/distribución; faltan atajos configurables por panel. |

Una suite correcta no implica que las funcionalidades pendientes estén
implementadas. Los detalles y limitaciones se conservan en cada issue.

## Issue 4 Agregar opciones por defecto para limitar total de la búsqueda

Tener la opción de poder elegir un límite por defecto a la totalidad de registros obtenidos, para no quedar esperando a que se muestren millones de registros en los casos que solo quería ver el cómo se llena una tabla.

### Consideraciones: carga incremental de resultados

La intención es cargar resultados por bloques bajo demanda, tomando como referencia el comportamiento descrito de DBeaver; no imponer un máximo definitivo de filas.

- Al ejecutar, por ejemplo, `SELECT * FROM tabla`, obtener y mostrar únicamente un primer bloque de filas, con un tamaño predeterminado configurable.
- Al desplazarse hacia el final de las filas cargadas, obtener el siguiente bloque para continuar explorando el resultado.
- Si el usuario no continúa desplazándose, no seguir descargando automáticamente todas las filas de la consulta.
- No basta con limitar las filas dibujadas en pantalla mientras se siguen obteniendo y almacenando millones de registros en segundo plano.

**Estado actual:** solucionada. Al ejecutar una consulta se muestran solo las primeras 1000 filas (valor configurable entre 1 y 100 000 en `result_block_size` o en **Ctrl+P → Result block size**). Al llegar al final de las filas cargadas con `↓`, `Ctrl+N`, `Av Pág`, `Fin`, `j` o la rueda del ratón, se solicita el siguiente bloque; sin ese desplazamiento no se descarga nada más.

La pestaña **no queda bloqueada** en ese estado: no permite dos sentencias a la vez sobre la misma conexión, pero puedes ejecutar otra consulta. Al hacerlo, la ejecución en pausa se cancela, se libera la conexión y recién entonces corre la nueva. Lo mismo aplica al invocar un objeto, cambiar de base, desconectar, cerrar la pestaña o salir. Mientras está en pausa el indicador deja de animarse y el tiempo mostrado se congela, para no aparentar que sigue trabajando.

`F10` cancela y conserva las filas recibidas. Mientras la lectura siga abierta, copiar, ordenar o exportar requieren pulsar `F10` primero.

**Limitación conocida:** la espera de liberación no tiene tiempo límite. Si un driver no responde a la cancelación, la pestaña permanece ocupada con el mensaje de liberación.

**Consideración técnica:** pausar la lectura de filas en el cliente no garantiza que el motor detenga el procesamiento de la consulta. El mecanismo para obtener los siguientes bloques debe definirse durante la implementación según el motor y el driver.

## Issue 5 Borrado por palabras: Ctrl + backspace

Que tenga la funcionalidad de poder borrar la palabra completa al presionar la combinación de teclas "ctrl + backspace" o "ctrl + supr/del"

### Consideraciones: no se implementará

Decisión del usuario: "issue 5 no se hara se mantendra ctrl+ w".

Se conserva **Ctrl+W** en modo INSERT (borra la palabra anterior), que ya funciona mediante el componente de edición. No se agregan `Ctrl+Backspace` ni `Ctrl+Delete`. Conviene documentar `Ctrl+W` en la ayuda, porque hoy existe pero no está anunciado.

**Estado actual:** cerrado como no se hará. No queda pendiente la implementación, solo la mención en la ayuda.

## Issue 6 Nueva opcion de copiar 

Permitir copiar 1 celda . 1 valor . con cabecera

### Consideraciones: formato de la celda con cabecera

Se copia el nombre de la columna y el valor de la celda seleccionada, en una sola fila separada por tabulador:

```
nombre_cliente<TAB>ACME S.A.
```

Al pegar en Excel u otra hoja de cálculo, el encabezado y el valor quedan en dos celdas contiguas de la misma fila.

- El modo seguro para hojas de cálculo se mantiene por defecto: si el valor empieza con un marcador de fórmula (`=`, `+`, `-`, `@`), se neutraliza igual que en la copia actual.
- La opción literal seguirá disponible como alternativa explícita.
- Un valor SQL `NULL` se representa igual que en el resto de la copia (`\N`).

**Estado actual:** solucionada. La opción **“Cell with header · column + value”** aparece en el menú de copia de resultados (**Ctrl+C**) justo después de copiar una celda simple, y como **“Copy column name and value”** en el inspector de la celda (**Enter** sobre un resultado). Ambas tienen variante literal explícita. El encabezado toma el nombre real de la columna del resultado, respetando el orden visible y la columna seleccionada.

**Nota:** al copiar una celda sola, el botón de valor continúa mostrando `SQL NULL` como texto de pantalla; esta opción nueva escribe `\N`, igual que el resto de la copia de filas y conjuntos.

## Issue 7 Correcion Resultado de sql 

Los valores tipo uniqueidentifier se ven con caracteres raros

**Estado actual:** solucionada. Las columnas SQL Server `UNIQUEIDENTIFIER` se
muestran como GUID legibles, por ejemplo `01234567-89AB-CDEF-0123-456789ABCDEF`,
respetando el orden de bytes del driver. `NULL`, otras columnas binarias y los
resultados de otros motores conservan su comportamiento. La corrección también
cubre procedimientos, varios conjuntos de resultados y carga por bloques.

**Verificación:** confirmada por el usuario; pruebas de regresión en
`internal/db/uniqueidentifier_test.go` y comparación de resultados no afectados.
Commit: `6adc2c1`.

## Issue 8 Error al copiar

No permite copiar cuando estoy seleccionando la vista del editor

### Consideraciones: copiar el SQL del editor

Se refiere a copiar el texto de la consulta, no los resultados. Ejemplo indicado: escribir `select * from tabla` en el editor y poder copiar ese SQL al portapapeles del sistema para pegarlo fuera de la aplicación.

- Con foco en el editor, `Ctrl+C` debe permitir copiar el SQL seleccionado, en lugar de abrir el menú de copia de resultados.
- La copia no debe modificar ni ejecutar la consulta.

**Estado actual:** solucionada mediante implementación y pruebas automatizadas.
Con foco en el editor, **Ctrl+C** copia únicamente la selección SQL al
portapapeles del sistema, sin abrir el menú de resultados ni modificar el texto,
la selección, el modo, el historial de deshacer, la ruta o el estado de guardado.
Se conserva el texto literal, incluidos Unicode y saltos de línea; no se agregan
cabeceras, formato TSV ni protección de fórmulas de hojas de cálculo. Un NUL se
representa como `\0` solo en el portapapeles y se avisa al usuario.

La copia es independiente de la ejecución SQL y funciona mientras una consulta
está en curso. Se realiza fuera del bucle de interfaz, con protección contra
copias SQL duplicadas y sin aplicar la notificación a otra pestaña si se cierra
la original. Los errores del portapapeles se muestran en la barra de estado.

En resultados, objetos y otros paneles, **Ctrl+C** conserva el menú de copia;
en un diálogo abierto conserva su función de cancelarlo. El comando Vim `y`
sigue usando su portapapeles interno, sin cambios.

**Verificación:** pruebas en `internal/ui/editor_clipboard_test.go` y
`internal/ui/copy_menu_test.go`; suite completa, `go vet`, detector de carreras y
comparación de los contextos no afectados contra la versión anterior. La
verificación independiente terminó correctamente. Las pruebas usan un escritor
de portapapeles inyectado: queda por comprobar manualmente el pegado desde una
terminal real en otra aplicación; no se atribuye confirmación funcional del
usuario a esta issue todavía.

**Decisión confirmada del usuario:** «Mostrar aviso y no copiar nada». Sin
selección se pide seleccionar texto SQL y se conserva el portapapeles existente;
no se copia automáticamente todo el documento.

## Issue 9  Atajo para comentar

agregar funcion que permita comentara cuando se esta en el editor.
1.- comentar lo seleccionado
2.- comentar todo el contenido del archivo

**Estado actual:** solucionada. **Alt+/** comenta o descomenta las líneas completas
alcanzadas por la selección. Si una línea ya tiene `--` después de la indentación,
se quita ese prefijo y un espacio opcional; si no, se agrega `-- `. En selecciones
mixtas se alterna cada línea por separado. La selección se conserva para repetir
el comando; sin selección solo se muestra un aviso y no se modifica el texto.

**Ctrl+P** sigue abriendo la paleta: contiene **Comentar selección** y
**Comentar todo el documento** como acciones separadas. Los comentarios de línea
se dibujan con el color atenuado del tema; mientras estén seleccionados conservan
los colores de selección. Se mantienen indentación, Unicode, saltos de línea y
deshacer/rehacer, sin ejecutar ni guardar SQL.

**Verificación:** confirmada por el usuario; pruebas en
`internal/editor/comments_test.go`, `internal/editor/comment_styles_test.go`,
`internal/ui/comments_test.go` e `internal/ui/comment_styles_test.go`.
Commit: `5f235db`.

## Issue 10 Pemitir abrir/buscar/guardar archivos sql de otras carpetas

favor en vez de buscar archivos y al guardar que se abra una ventana buscar archivos en distintanas partes, lo mismo para guardar que pueda guardar archivos en distintas partes

### Consideraciones: selector de archivos y Guardar como

La referencia es el flujo de **Guardar como** de VS Code: abrir un diálogo donde se pueda explorar carpetas y elegir dónde guardar el archivo, en lugar de tener que escribir manualmente su ruta completa.

- Al abrir un archivo SQL, permitir navegar entre carpetas y seleccionar el archivo.
- Ofrecer **Guardar como** para elegir la carpeta de destino y el nombre del archivo SQL mediante un diálogo.
- Permitir elegir ubicaciones distintas de la carpeta actual del proyecto.

**Estado actual:** solucionada. **Ctrl+O** abre un explorador integrado en la
terminal para navegar por carpetas y seleccionar archivos, también fuera del
proyecto. **Ctrl+P → Guardar SQL como** permite elegir carpeta y nombre de destino.
**Ctrl+S** guarda directamente si el documento ya tiene ruta; si no, abre el
selector. Se conservan la confirmación de sobrescritura, la protección de SQL sin
guardar y la restauración del foco al cancelar.

**Verificación:** confirmada por el usuario; pruebas en
`internal/ui/filepicker_test.go`, incluidos apertura, guardado, cancelación,
protecciones y aplicación del tema seleccionado. Commit: `76ff3c9`.

**Decisión confirmada:** explorador integrado en la terminal, no un diálogo del
sistema operativo. No se crean carpetas ni se recuerda la última ubicación entre
sesiones; esas capacidades no formaban parte del alcance acordado.

## Issue 11 Atajos adicionales

Agregar unos atajos configurables que permitan ocultar/mostrar los distintos paneles

**Estado actual:** parcial; todavía no está solucionada como fue solicitada.
Existe **F11 → Distribución**, con opciones para ocultar/mostrar el catálogo,
maximizar el editor o los resultados y guardar preferencias de distribución.
Sin embargo, faltan acciones de teclado configurables para ocultar/mostrar cada
panel individualmente.

**Evidencia:** `internal/ui/workspace.go` contiene los controles de distribución;
`internal/ui/keys.go` no reconoce acciones de alternancia por panel. La prueba
`TestResponsiveLayoutPreservesSQLAndSavedPreferences` cubre la distribución
actual, no los atajos configurables solicitados.

**Pendiente:** definir los paneles y atajos configurables que se ofrecerán e
implementar sus acciones, persistencia, ayuda y pruebas.