# Guia de Uso do Weaver

## Visão Geral

O Weaver ajuda a gerenciar stacks de branches localmente. Você declara as dependências uma vez e usa essas declarações para inspeção, rebase, composição e exportação/importação do estado.

## Configuração

Compile o binário:

```bash
make build
```

Inicialize o Weaver dentro do repositório Git:

```bash
./bin/weaver init
```

Isso cria:

- `.weaver.yaml`
- `.git/weaver/`

## Declarar um Stack

Declare que `feature-b` depende de `feature-a`:

```bash
./bin/weaver stack feature-b --on feature-a
```

Adicione outra branch no topo:

```bash
./bin/weaver stack feature-c --on feature-b
```

Mude a branch para outro pai:

```bash
./bin/weaver stack feature-c --on main
```

Remova a declaração de dependência:

```bash
./bin/weaver unstack feature-c
```

## Inspecionar Dependências

Mostre a árvore completa:

```bash
./bin/weaver deps
```

Mostre a cadeia de uma branch:

```bash
./bin/weaver deps feature-c
```

Saída típica:

```text
main -> feature-a -> feature-b -> feature-c
```

## Verificar a Saúde do Stack

Mostre a árvore com o estado de cada branch:

```bash
./bin/weaver status
```

Estados possíveis:

- `clean`
- `needs rebase`
- `conflict risk`

## Rebase de um Stack

Faça rebase de todo o stack até `feature-c`:

```bash
./bin/weaver sync feature-c
```

Se você já estiver na branch alvo:

```bash
./bin/weaver sync
```

Se houver conflito:

```bash
./bin/weaver continue
./bin/weaver abort
```

`continue` retoma depois da resolução manual. `abort` cancela a operação e volta para a branch original.

## Compor Branches

Faça um dry-run:

```bash
./bin/weaver compose feature-c --dry-run
```

Componha várias branches:

```bash
./bin/weaver compose feature-a feature-c feature-e
```

Componha todas as branches rastreadas:

```bash
./bin/weaver compose --all
```

A composição é efêmera por padrão. O comando usa `HEAD` destacado, faz os merges e retorna para a branch original.

Se você quiser atualizar uma branch real de integração com o resultado composto, faça opt-in explícito:

```bash
./bin/weaver compose feature-b feature-d --base integration --persist
```

Isso atualiza `integration` para o commit composto e depois volta para a branch original.

## Gerenciar Grupos

Crie um grupo:

```bash
./bin/weaver group create sprint-42 feature-a feature-b feature-c
```

Adicione branches:

```bash
./bin/weaver group add sprint-42 feature-d feature-e
```

Remova branches de um grupo:

```bash
./bin/weaver group remove sprint-42 feature-c
```

Apague o grupo inteiro:

```bash
./bin/weaver group remove sprint-42
```

Liste os grupos:

```bash
./bin/weaver group list
```

Componha um grupo:

```bash
./bin/weaver compose --group sprint-42
```

## Exportar e Importar

Exporte o estado local:

```bash
./bin/weaver export > weaver-state.json
```

Importe em outro clone:

```bash
./bin/weaver import weaver-state.json
```

## Smoke Test

Execute o script de verificação ponta a ponta:

```bash
./smoketest.sh
```

Ele grava um log passo a passo em `smoketest.log`.
