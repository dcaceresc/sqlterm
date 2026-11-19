# SQLTerm

SQLTerm es un cliente SQL de terminal escrito en Go para **SQL Server, MySQL y PostgreSQL**. Lo creé para mi flujo personal: trabajo principalmente en Linux y necesito usar la misma herramienta también en Windows para buscar objetos, revisar y editar rutinas, ejecutar consultas y explorar resultados sin salir de la terminal.

No pretende ser un cliente universal ni reemplazar todas las herramientas de administración de bases de datos. Su alcance prioriza las capacidades que uso y puedo mantener con claridad.

## Características principales

- Perfiles de conexión organizados por carpetas, favoritos y entorno.
- Catálogo de tablas, vistas, procedimientos y funciones.
- Editor integrado con modos Vim básicos, pestañas y archivos SQL.
- Invocación parametrizada y edición de procedimientos y funciones según el motor.
- IntelliSense basado en el dialecto, el catálogo y las referencias simples del SQL.
- Ejecución cancelable con múltiples conjuntos de resultados y recepción incremental.
- Copia segura, inspección de celdas y exportación CSV o JSON.
- Temas, atajos configurables e historial local opcional.

## Plataformas

Linux y Windows son las plataformas principales. Go permite compilar el proyecto para ambas, y la automatización también genera binarios para macOS como destino adicional de compilación. Esto no implica que todas las funciones se hayan probado en vivo en cada sistema: el terminal, el portapapeles, el almacén de credenciales, las dependencias de la sesión, la red y los servidores disponibles también afectan la ejecución.

## Instalación y uso

Compilar desde el código fuente requiere **Go 1.24 o posterior**. El backend Windows actualizado negocia pegado delimitado con terminales compatibles; las pruebas automáticas del parser no sustituyen la comprobación interactiva de Windows Terminal.

Sigue la [instalación recomendada](INSTRUCTIONS.md#instalación-recomendada) para instalar una vez desde el repositorio y después ejecutar `sqlterm` desde cualquier carpeta. La guía también cubre configuración, atajos, automatización, pruebas y límites actuales.
