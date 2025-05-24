# go-batect

## Overview

go-batect is a lightweight Go implementation of the Batect tool. It allows you to run tasks in isolated Docker containers using a `batect.yml` or `config.yml` configuration file. This ensures that your development environment and CI tasks are consistent across all machines.

## Requirements

- Docker
- Docker Compose (plugin)
- Linux, Windows, or macOS

## Code

#### Prereq

You will need Go installed if you want to run from source or compile the binaries yourself.

```bash
go mod tidy
```

#### Task Configuration

Create a `batect.yml` in your project root.

```yaml
containers:
  bash:
    image: bash:latest
    volumes:
      - local: .
        container: /code
    working_directory: /code

tasks:
  hello:
    description: "Prints hello world from a container"
    run:
      container: bash
      command: echo "Hello World"
```

#### Running Tasks

To run a task defined in your config:

```bash
go run main.go hello
```

To list all available tasks:

```bash
go run main.go --list
```

#### Docker Build with Buildx

You can define a build context for a container. By default, it uses `buildx` with a fallback to legacy `docker build`.

```yaml
containers:
  app-builder:
    build: ./docker
    legacy_build: false # Set to true to force legacy builder
```

#### Docker Compose

You can also run tasks using Docker Compose.

```yaml
tasks:
  test:
    description: "Run tests in compose environment"
    docker_compose: true
    docker_compose_file: docker-compose.yml
    docker_compose_down: true
    run:
      container: nodeapp
      command: npm test
```

## Compilation

Binaries can be compiled for multiple platforms. These are placed in the `binaries/` folder.

```bash
# Using go-batect to compile itself (see config.yml)
go run main.go compile
```

## Features

- **Isolated Tasks**: Every task runs in its own container.
- **Dependency Management**: Tasks can have prerequisites.
- **Healthchecks**: Wait for containers to be healthy before running commands.
- **Buildx Support**: Modern Docker build capabilities with automatic fallback.
- **Cross-Platform**: Compiled for Linux and Windows (x86_64 and ARM64).