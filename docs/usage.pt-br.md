# Guia de Uso do Weaver

## Visão Geral

O Weaver ajuda a gerenciar stacks de branches localmente. Você declara as dependências uma vez e usa essas declarações para inspeção, rebase, composição e exportação/importação do estado.

## Configuração

Instale a release estável mais recente:

```bash
go install github.com/lutefd/weaver@latest
weaver version
```

Inicialize o Weaver dentro do repositório Git:

```bash
weaver init
```

Isso cria:

- `.weaver.yaml`
- `.git/weaver/`

## Declarar um Stack

Declare que `feature-b` depende de `feature-a`:

```bash
weaver stack feature-b --on feature-a
```

Adicione outra branch no topo:

```bash
weaver stack feature-c --on feature-b
```

Mude a branch para outro pai:

```bash
weaver stack feature-c --on main
```

Remova a declaração de dependência:

```bash
weaver unstack feature-c
```

## Inspecionar Dependências

Mostre a árvore completa:

```bash
weaver deps
```

Mostre a cadeia de uma branch:

```bash
weaver deps feature-c
```

Saída típica:

```text
main -> feature-a -> feature-b -> feature-c
```

## Verificar a Saúde do Stack

Mostre a árvore com o estado de cada branch:

```bash
weaver status
```

Estados possíveis:

- `clean`
- `needs rebase`
- `conflict risk`

## Diagnosticar o Estado Local

Rode uma verificação somente leitura:

```bash
weaver doctor
```

Saída legível por máquina:

```bash
weaver doctor --json
```

`weaver doctor` verifica inicialização, config, branches declaradas, estado pendente de rebase e problemas comuns de Git, como árvore suja ou operações em andamento.

## Atualizar Branches Locais a Partir do Upstream

Atualize branches explícitas:

```bash
weaver update main feature-a feature-b
```

Atualize todas as branches rastreadas:

```bash
weaver update --all
```

Atualize um grupo nomeado:

```bash
weaver update --group sprint-42
```

Atualize todas as branches rastreadas por uma estratégia de integração salva:

```bash
weaver update --integration integration
```

`weaver update` roda `git fetch --all` uma vez e depois faz fast-forward de cada branch local selecionada até o upstream configurado. O comando para se uma branch não tiver upstream ou não puder receber fast-forward.

## Rebase de um Stack

Faça rebase de todo o stack até `feature-c`:

```bash
weaver sync feature-c
```

Se você já estiver na branch alvo:

```bash
weaver sync
```

Se houver conflito:

```bash
weaver continue
weaver abort
```

`continue` retoma depois da resolução manual. `abort` cancela a operação e volta para a branch original.

## Compor Branches

Faça um dry-run:

```bash
weaver compose feature-c --dry-run
```

Componha várias branches:

```bash
weaver compose feature-a feature-c feature-e
```

Componha todas as branches rastreadas:

```bash
weaver compose --all
```

Pule uma branch problemática e mantenha o restante da composição:

```bash
weaver compose --integration integration --create integration-preview --skip feature-debug-search-api-curl
```

A composição é efêmera por padrão. O comando usa `HEAD` destacado, faz os merges e retorna para a branch original.

Se você quiser criar uma nova branch de integração a partir do resultado composto, faça opt-in explícito:

```bash
weaver compose feature-b feature-d --base main --create integration
```

Se você quiser recriar uma branch de integração existente a partir da base limpa, faça opt-in explícito:

```bash
weaver compose feature-b feature-d --base main --update integration
```

Com `--create`, o Weaver cria `integration` a partir do commit composto e depois volta para a branch original.

Com `--update`, o Weaver parte de `main`, recompõe as branches pedidas, move `integration` à força para esse resultado novo e depois volta para a branch original.

Se a composição encontrar conflito, o Weaver informa qual branch falhou e quais arquivos entraram em conflito.

Se você não tiver passado `--skip`, o Weaver pergunta se quer pular a branch com problema ou abortar a composição.

Se uma branch estiver muito divergente e continuar quebrando uma composição grande, normalmente é melhor remover essa branch da composição ou da integração salva, corrigi-la primeiro, e depois mergeá-la manualmente na branch produzida por `--create` ou `--update` antes de colocá-la de volta quando estiver estável.

Se você já tiver salvo uma estratégia reutilizável, pode compor direto dela:

```bash
weaver compose --integration integration --update integration
```

Ao usar `--integration`, o Weaver pega a base e a lista de branches da estratégia salva.

## Gerenciar Integrações Salvas

Salve ou atualize uma estratégia:

```bash
weaver integration save integration --base main feature-a feature-b feature-c
```

Mostre a estratégia:

```bash
weaver integration show integration
```

Diagnostique a estratégia:

```bash
weaver integration doctor integration
weaver integration doctor integration --json
```

Liste as estratégias salvas:

```bash
weaver integration list
```

Remova uma estratégia:

```bash
weaver integration remove integration
```

Exporte uma estratégia em JSON:

```bash
weaver integration export integration --json > integration.json
```

Importe em outro clone:

```bash
weaver integration import integration.json
```

## Gerenciar Grupos

Crie um grupo:

```bash
weaver group create sprint-42 feature-a feature-b feature-c
```

Adicione branches:

```bash
weaver group add sprint-42 feature-d feature-e
```

Remova branches de um grupo:

```bash
weaver group remove sprint-42 feature-c
```

Apague o grupo inteiro:

```bash
weaver group remove sprint-42
```

Liste os grupos:

```bash
weaver group list
```

Componha um grupo:

```bash
weaver compose --group sprint-42
```

## Exportar e Importar

Exporte o estado local, incluindo integrações salvas:

```bash
weaver export > weaver-state.json
```

Importe em outro clone:

```bash
weaver import weaver-state.json
```

## Smoke Test

Execute o script de verificação ponta a ponta:

```bash
./smoketest.sh
```

Ele grava um log passo a passo em `smoketest.log`.
