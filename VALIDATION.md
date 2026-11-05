# Validación local

Fecha: 2026-11-05. Entorno: Windows amd64, Go 1.27.3.
El módulo declara Go 1.23; el workflow usa Go 1.23.x.

- `go test ./...`: aprobado. Incluye lexer SQL, generación parametrizada,
  streaming con varios conjuntos, cancelación, errores, paginación de 10.000
  filas, CSV/JSON con NULL/Unicode, persistencia y controles Vim.
- `go vet ./...`: aprobado.
- TUI con pantalla tcell simulada: aprobado; paneles, máscaras de contraseña,
  rutas visibles, acceso al botón de aplicar con teclado y cierre con tareas.
- Compilación sin CGO: Windows, Linux y macOS, para amd64 y arm64.
- CLI de Windows: verificación de `--version` y `--help`.
- Consola Windows real mediante PTY: arranque, pantalla de conexiones, vuelta al
  editor con Esc y cierre limpio con Ctrl+C (código de salida 0).

Actualización 0.1.2: pruebas de handshake TLS contra un servidor local con
certificado no confiable (rechazo predeterminado, confianza explícita y CA
configurada), formulario de contraseña oculta, opciones de cifrado/confianza,
recuperación de secretos tras reiniciar, reemplazo y reversión si falla el guardado.
Las pruebas de credenciales usan un almacén en memoria; no modifican el almacén
de contraseñas real del equipo.

Actualización 0.1.3: puertos predeterminados visibles por motor, actualización al
cambiar de motor y conservación de puertos personalizados. Regresión del catálogo
SQL Server con un driver simulado que rechaza `STRING_AGG` y `WITHIN GROUP`;
comprueba firmas ordenadas, objetos sin parámetros, Unicode y funciones escalares.
No se dispuso de una instancia antigua de SQL Server para validación real.

Actualización 0.1.4: vista previa, cancelación, persistencia y fallo de guardado de
temas; colores de editor y resultados sin cambiar el SQL, el cursor ni los conjuntos
abiertos; formularios con contraseña oculta en las seis paletas. Se comprueba que
no se modifican los estilos globales de tview.

Carpetas: árbol con subcarpetas y filtro, índices correctos después de ordenar,
creación de perfiles en la carpeta seleccionada, renombrado de árboles, movimiento
de perfiles, bloqueo de borrado de carpetas con conexiones y aislamiento de
contraseñas entre perfiles del mismo nombre en DEV y PROD. Se verifica persistencia
y compatibilidad con los perfiles anteriores sin carpeta.
Prueba nativa de Windows mediante PTY: árbol DEV/QA/PROD, apertura del selector
con F3, vista previa, selección de Nord y guardado en una configuración temporal
separada de los perfiles del usuario; salida limpia con código 0.

Actualización 0.1.5: transiciones NORMAL/INSERT/VISUAL en las seis paletas;
verificación del color real de la celda del indicador, del borde activo y de las
solicitudes de forma/color del cursor. Se comprueba restauración con Esc,
cambio de tema, foco, formularios y pestañas, y ajuste de la barra a diferentes
anchos con texto Unicode, manteniendo visible la posición del cursor.
Prueba nativa de Windows mediante PTY: NORMAL azul con cursor de bloque,
INSERT verde con cursor de barra, vuelta con Esc y VISUAL violeta; se observaron
las secuencias de cursor correspondientes y la salida terminó con código 0.

Actualización 0.1.6: copia TSV con/sin cabeceras, orden visible, NULL y caracteres
especiales; prueba de portapapeles mediante un backend simulado, incluidos errores.
Perfiles sin BD: enumeración en el explorador, bloqueo del SQL en la base de entrada,
selección con F4 y posterior carga de objetos, sin modificar el perfil persistido.

Test connection: datos sin guardar, contraseña escrita o recuperada, cierre de la
conexión temporal, cancelación y errores sin exponer contraseñas. IntelliSense:
alias, identificadores entre comillas, dialectos, parámetros de procedimientos,
exclusión de comentarios/cadenas, foco de escritura conservado y aceptación con Tab.
Las pruebas de flujos de conexión usan drivers simulados, sin servidores reales.
Prueba nativa Windows mediante PTY: sugerencia automática de SELECT y aceptación
con Tab conservando INSERT; botón Test connection visible en 80×24, respuesta de
error al probar 127.0.0.1:1 y salida limpia. No se creó el archivo de configuración
temporal, confirmando que la prueba no guardó el perfil.

Actualización 0.1.7: reproducido el panic de la conversión UTF-16 de Windows con
filas TSV que contienen NUL, sin modificar el portapapeles del usuario. Pruebas de
copia de filas y conjuntos con/sin cabeceras, escape de NUL sin alterar los datos,
y recuperación de un panic del backend seguida de otra copia en la misma UI.

## Pendiente en entornos externos

No se proporcionaron servidores de prueba ni credenciales; Docker no está
disponible. Las tres variantes de `TestIntegration` se omitieron explícitamente.
Los mocks verifican coordinación y manejo de resultados, no el protocolo real
de los servidores. El workflow incluye servicios SQL Server 2022, MySQL 8 y
PostgreSQL 16 para ejecutar esos casos cuando se publique el repositorio.

No se ejecutó una sesión interactiva nativa en Linux/macOS, ni una prueba real
de TLS contra servidores. La compilación cruzada no sustituye esas verificaciones.
La prueba con detector de carreras está configurada en CI Linux; no se ejecutó
localmente por no disponer de un compilador C para CGO.

Las pruebas de integración crean, encuentran, leen, invocan, modifican y eliminan
rutinas; comprueban parámetros de salida, retorno de SQL Server, conjuntos
múltiples y cancelación. Usan únicamente las bases indicadas por las variables
`SQLTERM_TEST_*_PROFILE` y `SQLTERM_TEST_*_PASSWORD`.
