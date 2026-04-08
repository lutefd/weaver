# Guía de Uso de Weaver

## Resumen

Weaver ayuda a manejar stacks de branches de forma local. Declarás las dependencias una vez y después usás esas relaciones para inspección, rebase, composición y exportación/importación del estado.

## Preparación

Instalá la última release estable:

```bash
go install github.com/lutefd/weaver@latest
weaver version
```

Inicializá Weaver dentro del repositorio Git:

```bash
weaver init
```

Esto crea:

- `.weaver.yaml`
- `.git/weaver/`

## Declarar un Stack

Declarar que `feature-b` depende de `feature-a`:

```bash
weaver stack feature-b --on feature-a
```

Agregar otra branch arriba:

```bash
weaver stack feature-c --on feature-b
```

Mover una branch a otro padre:

```bash
weaver stack feature-c --on main
```

Eliminar una dependencia:

```bash
weaver unstack feature-c
```

## Inspeccionar Dependencias

Mostrar el árbol completo:

```bash
weaver deps
```

Mostrar la cadena de una branch:

```bash
weaver deps feature-c
```

Salida típica:

```text
main -> feature-a -> feature-b -> feature-c
```

## Ver el Estado del Stack

Mostrar el árbol con el estado de cada branch:

```bash
weaver status
```

Etiquetas posibles:

- `clean`
- `needs rebase`
- `conflict risk`

## Diagnosticar el Estado Local

Ejecutá una revisión de solo lectura:

```bash
weaver doctor
```

Salida legible por máquina:

```bash
weaver doctor --json
```

`weaver doctor` revisa la inicialización, la config, las branches declaradas, el estado pendiente de rebase y problemas comunes de Git, como working tree sucio u operaciones en curso.

## Actualizar Branches Locales Desde el Upstream

Actualizar branches explícitas:

```bash
weaver update main feature-a feature-b
```

Actualizar todas las branches registradas:

```bash
weaver update --all
```

Actualizar un grupo nombrado:

```bash
weaver update --group sprint-42
```

Actualizar todas las branches rastreadas por una estrategia de integración guardada:

```bash
weaver update --integration integration
```

`weaver update` corre `git fetch --all` una vez y después hace fast-forward de cada branch local seleccionada hasta su upstream configurado. El comando se frena si una branch no tiene upstream o no puede avanzar con fast-forward.

## Rebase de un Stack

Hacer rebase de todo el stack que termina en `feature-c`:

```bash
weaver sync feature-c
```

Si ya estás parado en la branch objetivo:

```bash
weaver sync
```

Si el proceso se frena por conflictos:

```bash
weaver continue
weaver abort
```

`continue` retoma después de resolver conflictos manualmente. `abort` cancela la operación y vuelve a la branch original.

## Componer Branches

Hacer un dry-run:

```bash
weaver compose feature-c --dry-run
```

Componer varias branches:

```bash
weaver compose feature-a feature-c feature-e
```

Componer todas las branches registradas:

```bash
weaver compose --all
```

Saltar una branch problemática y mantener el resto de la composición:

```bash
weaver compose --integration integration --create integration-preview --skip feature-debug-search-api-curl
```

La composición es efímera por defecto. Usa `HEAD` detached, hace los merges y vuelve a la branch original.

Si querés crear una nueva branch de integración a partir del resultado compuesto, hacelo de forma explícita:

```bash
weaver compose feature-b feature-d --base main --create integration
```

Si querés reconstruir una branch de integración existente desde la base limpia, hacelo de forma explícita:

```bash
weaver compose feature-b feature-d --base main --update integration
```

Con `--create`, Weaver crea `integration` desde el commit compuesto y después vuelve a la branch original.

Con `--update`, Weaver parte de `main`, recompone las branches pedidas, mueve `integration` por fuerza a ese resultado nuevo y después vuelve a la branch original.

Si la composición encuentra un conflicto, Weaver informa qué branch falló y qué archivos entraron en conflicto.

Si no pasaste `--skip`, Weaver te pregunta si querés saltar la branch problemática o abortar la composición.

Si una branch está muy divergida y sigue rompiendo una composición grande, normalmente conviene sacarla de esa composición o de la integración guardada, arreglarla primero, y después mergearla manualmente en la branch producida por `--create` o `--update` antes de volver a sumarla cuando esté estable.

Si ya guardaste una estrategia reutilizable, podés componer directo desde ahí:

```bash
weaver compose --integration integration --update integration
```

Cuando usás `--integration`, Weaver toma la base y la lista de branches de la estrategia guardada.

## Manejar Integraciones Guardadas

Guardar o actualizar una estrategia:

```bash
weaver integration save integration --base main feature-a feature-b feature-c
```

Mostrar la estrategia:

```bash
weaver integration show integration
```

Diagnosticar la estrategia:

```bash
weaver integration doctor integration
weaver integration doctor integration --json
```

Listar las estrategias guardadas:

```bash
weaver integration list
```

Eliminar una estrategia:

```bash
weaver integration remove integration
```

Exportar una estrategia como JSON:

```bash
weaver integration export integration --json > integration.json
```

Importarla en otro clon:

```bash
weaver integration import integration.json
```

## Manejar Grupos

Crear un grupo:

```bash
weaver group create sprint-42 feature-a feature-b feature-c
```

Agregar más branches:

```bash
weaver group add sprint-42 feature-d feature-e
```

Sacar branches de un grupo:

```bash
weaver group remove sprint-42 feature-c
```

Borrar el grupo completo:

```bash
weaver group remove sprint-42
```

Listar grupos:

```bash
weaver group list
```

Componer un grupo:

```bash
weaver compose --group sprint-42
```

## Exportar e Importar

Exportar el estado local, incluyendo integraciones guardadas:

```bash
weaver export > weaver-state.json
```

Importarlo en otro clon:

```bash
weaver import weaver-state.json
```

## Smoke Test

Ejecutá el script de verificación end-to-end:

```bash
./smoketest.sh
```

Genera un log paso a paso en `smoketest.log`.
