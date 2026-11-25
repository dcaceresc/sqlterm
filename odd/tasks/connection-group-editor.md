# Connection group editor
Status: implementation and local checks passed; independent verification could not run.
## Specs
S1. “si yo quiero cambiar el nombre de una carpeta no puedo el boton de editar conexion deberia pasar a boton editar si el usuario hace e estando sobre el nombre de la carpeta deberia poder editar su nombre”
S2. “y deberia aparecer un select que diga parent group que haga si la carpeta cambia a ser una subcarpeta o queda en la raiz”
S3. “tambien cada carpeta debe tener un color me fije que si el usuario elige la conexion de la carpeta prod aparece de color rojo pero eso deberia ser un color que se seleccione cuando el usuario cree la carpeta o la edite”. Selected option: “Conservar los colores actuales”.
S4. “las opciones hay rojo aranjado y amarillo deja el rojo, naranja y amarillo”. Clarification: “quiero que agregues el color rojo y naranja”.
## Tasks
T1 | S1–S3 | Local checks passed; candidate `65de5de` | Contextual group editor, tree-safe moves and persistent color; preserve current confirmation policy. Code, tests and bilingual help in one behavior unit.
T2 | S4 | Local tests passed; uncommitted | Add separate Red and Orange choices without removing existing colors.
## Log
L1. “si yo quiero cambiar el nombre de una carpeta no puedo el boton de editar conexion deberia pasar a boton editar si el usuario hace e estando sobre el nombre de la carpeta deberia poder editar su nombre y deberia aparecer un select que diga parent group que haga si la carpeta cambia a ser una subcarpeta o queda en la raiz tambien cada carpeta debe tener un color me fije que si el usuario elige la conexion de la carpeta prod aparece de color rojo pero eso deberia ser un color que se seleccione cuando el usuario cree la carpeta o la edite”
L2. Selected option: “Conservar los colores actuales”. The earlier unstaged a/f shortcut changes must be preserved and not bundled as part of this feature.
L3. Focused UI/config regression checks, the complete Go suite, and isolated feature-only suite passed. The differential check for existing connection shortcuts matched stdout, stderr and exit status. Native assessment could not find the package-local binary.
L4. Candidate `65de5de` contains only the folder-editor feature (546 changed lines including tests and docs); earlier a/f edits remain unstaged. The linked checkout was not accepted by the verifier (`unsupported repository: ordinary single-root Git directory required`, exit 1, zero tests).
L5. “las opciones hay rojo aranjado y amarillo deja el rojo, naranja y amarillo”. Clarification: “quiero que agregues el color rojo y naranja”. The new Spanish labels and selected workspace colors passed the focused regression and `go test ./...`. The existing colors were preserved; native assessment remained unavailable.
