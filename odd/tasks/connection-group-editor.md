# Connection group editor
Status: implementation and local checks passed; awaiting independent verification.
## Specs
S1. “si yo quiero cambiar el nombre de una carpeta no puedo el boton de editar conexion deberia pasar a boton editar si el usuario hace e estando sobre el nombre de la carpeta deberia poder editar su nombre”
S2. “y deberia aparecer un select que diga parent group que haga si la carpeta cambia a ser una subcarpeta o queda en la raiz”
S3. “tambien cada carpeta debe tener un color me fije que si el usuario elige la conexion de la carpeta prod aparece de color rojo pero eso deberia ser un color que se seleccione cuando el usuario cree la carpeta o la edite”. Selected option: “Conservar los colores actuales”.
## Tasks
T1 | S1–S3 | Local checks passed; `feat: permite editar grupos de conexiones` | Contextual group editor, tree-safe moves and persistent color; preserve current confirmation policy. Code, tests and bilingual help in one behavior unit.
## Log
L1. “si yo quiero cambiar el nombre de una carpeta no puedo el boton de editar conexion deberia pasar a boton editar si el usuario hace e estando sobre el nombre de la carpeta deberia poder editar su nombre y deberia aparecer un select que diga parent group que haga si la carpeta cambia a ser una subcarpeta o queda en la raiz tambien cada carpeta debe tener un color me fije que si el usuario elige la conexion de la carpeta prod aparece de color rojo pero eso deberia ser un color que se seleccione cuando el usuario cree la carpeta o la edite”
L2. Selected option: “Conservar los colores actuales”. The earlier unstaged a/f shortcut changes must be preserved and not bundled as part of this feature.
L3. Focused UI/config regression checks, the complete Go suite, and isolated feature-only suite passed. The differential check for existing connection shortcuts matched stdout, stderr and exit status. Native assessment could not find the package-local binary.
