# Guía de Uso de Weaver

## Resumen

Weaver ayuda a manejar stacks de branches de forma local. Declarás las dependencias una vez y después usás esas relaciones para inspección, rebase, composición y exportación/importación del estado.

## Preparación

Compilá el binario:

```bash
make build
```

Inicializá Weaver dentro del repositorio Git:

```bash
./bin/weaver init
```

Esto crea:

- `.weaver.yaml`
- `.git/weaver/`

## Declarar un Stack

Declarar que `feature-b` depende de `feature-a`:

```bash
./bin/weaver stack feature-b --on feature-a
```

Agregar otra branch arriba:

```bash
./bin/weaver stack feature-c --on feature-b
```

Mover una branch a otro padre:

```bash
./bin/weaver stack feature-c --on main
```

Eliminar una dependencia:

```bash
./bin/weaver unstack feature-c
```

## Inspeccionar Dependencias

Mostrar el árbol completo:

```bash
./bin/weaver deps
```

Mostrar la cadena de una branch:

```bash
./bin/weaver deps feature-c
```

Salida típica:

```text
main -> feature-a -> feature-b -> feature-c
```

## Ver el Estado del Stack

Mostrar el árbol con el estado de cada branch:

```bash
./bin/weaver status
```

Etiquetas posibles:

- `clean`
- `needs rebase`
- `conflict risk`

## Actualizar Branches Locales Desde el Upstream

Actualizar branches explícitas:

```bash
./bin/weaver update main feature-a feature-b
```

Actualizar todas las branches registradas:

```bash
./bin/weaver update --all
```

Actualizar un grupo nombrado:

```bash
./bin/weaver update --group sprint-42
```

`weaver update` corre `git fetch --all` una vez y después hace fast-forward de cada branch local seleccionada hasta su upstream configurado. El comando se frena si una branch no tiene upstream o no puede avanzar con fast-forward.

## Rebase de un Stack

Hacer rebase de todo el stack que termina en `feature-c`:

```bash
./bin/weaver sync feature-c
```

Si ya estás parado en la branch objetivo:

```bash
./bin/weaver sync
```

Si el proceso se frena por conflictos:

```bash
./bin/weaver continue
./bin/weaver abort
```

`continue` retoma después de resolver conflictos manualmente. `abort` cancela la operación y vuelve a la branch original.

## Componer Branches

Hacer un dry-run:

```bash
./bin/weaver compose feature-c --dry-run
```

Componer varias branches:

```bash
./bin/weaver compose feature-a feature-c feature-e
```

Componer todas las branches registradas:

```bash
./bin/weaver compose --all
```

La composición es efímera por defecto. Usa `HEAD` detached, hace los merges y vuelve a la branch original.

Si querés crear una nueva branch de integración a partir del resultado compuesto, hacelo de forma explícita:

```bash
./bin/weaver compose feature-b feature-d --base main --create integration
```

Si querés reconstruir una branch de integración existente desde la base limpia, hacelo de forma explícita:

```bash
./bin/weaver compose feature-b feature-d --base main --update integration
```

Con `--create`, Weaver crea `integration` desde el commit compuesto y después vuelve a la branch original.

Con `--update`, Weaver parte de `main`, recompone las branches pedidas, mueve `integration` por fuerza a ese resultado nuevo y después vuelve a la branch original.

## Manejar Grupos

Crear un grupo:

```bash
./bin/weaver group create sprint-42 feature-a feature-b feature-c
```

Agregar más branches:

```bash
./bin/weaver group add sprint-42 feature-d feature-e
```

Sacar branches de un grupo:

```bash
./bin/weaver group remove sprint-42 feature-c
```

Borrar el grupo completo:

```bash
./bin/weaver group remove sprint-42
```

Listar grupos:

```bash
./bin/weaver group list
```

Componer un grupo:

```bash
./bin/weaver compose --group sprint-42
```

## Exportar e Importar

Exportar el estado local:

```bash
./bin/weaver export > weaver-state.json
```

Importarlo en otro clon:

```bash
./bin/weaver import weaver-state.json
```

## Smoke Test

Ejecutá el script de verificación end-to-end:

```bash
./smoketest.sh
```

Genera un log paso a paso en `smoketest.log`.
