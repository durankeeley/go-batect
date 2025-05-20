# go-batect

**go-batect** is a lightweight, single-binary replacement for the now-archived [Batect](https://github.com/batect/batect), completely rewritten in Go. It allows you to define and run isolated, repeatable tasks in Docker or Docker Compose using a simple, declarative `batect.yml` file.

Inspired by the *Environment as Code (EaC)* philosophy, `go-batect` simplifies onboarding, testing, and CI workflows by executing tasks inside reproducible, containerised environments.


## Key Features

* **Single static binary** – no runtime dependencies (just Docker).
* **Supports Docker and Docker Compose** workflows.
* **Declarative YAML**-based task definitions.
* **Per-command isolation** – consistent behaviour across machines.
* **Custom config support** via `--file`.
* **Auto-cleanup for Compose tasks** (optional).


## Requirements

* Docker (with Compose plugin)
* Linux, macOS, or Windows
* No need to install Go or other runtimes – all tasks run in containers.

## Why Use `go-batect`?

**Without `go-batect`:**

* Manual setup and onboarding pain
* Inconsistent local development environments
* Fragile test environments reliant on host configuration
* Tribal knowledge and out-of-date setup docs

**With `go-batect`:**

* Declarative, reproducible dev and CI environments
* Dockerised workflows with minimal setup
* Built-in Docker Compose and task dependency handling
* Easy team onboarding – works on *any* machine with Docker

## Task Execution Behaviour

Each task can define a `run` block, `prerequisites`, or both.

* If `run` is provided → the command will be executed.
* If only `prerequisites` are defined → each prerequisite task is run in order.
* If both `run` and `prerequisites` exist → prerequisites run **first**, then the task's `run` block.
* Tasks may use:

  * A prebuilt Docker `image`
  * A custom Docker `build` context (Dockerfile)
  * A Docker Compose service (`docker_compose: true`)


## Running Tasks

By default, `go-batect` looks for `batect.yml` or `config.yml` in the current directory.

```bash
./go-batect <task-name>
```

Run using a different file:

```bash
./go-batect test --file batect.ci.yml
```

## Listing All Tasks

```bash
./go-batect --list
```

Lists all available tasks from the active or specified `batect.yml`.


## Example: Basic Docker Image Task

```yaml
containers:
  terraform:
    image: hashicorp/terraform:latest
    volumes:
      - local: .
        container: /code
    working_directory: /code

tasks:
  validate:
    description: Validate Terraform code
    run:
      container: terraform
      command: validate
```

```bash
./go-batect validate
```

## Example: Custom Build with Dockerfile

**Dockerfile** (`./docker/Dockerfile`):

```Dockerfile
FROM golang:1.22
WORKDIR /app
COPY . .
RUN go build -o app .
```

**batect.yml**:

```yaml
containers:
  builder:
    build: ./docker
    working_directory: /app
    volumes:
      - local: .
        container: /app

tasks:
  build-app:
    description: Build Go app using Dockerfile
    run:
      container: builder
      command: go build -o mybinary
```

```bash
./go-batect build-app
```

> `build` points to a folder containing a `Dockerfile`.


## Example: Docker Compose Task

**docker-compose.yml**:

```yaml
services:
  nodeapp:
    image: node:20
    working_dir: /app
    command: sh -c "npm install && npm start"
    volumes:
      - .:/app
    environment:
      - NODE_ENV=development
```

**batect.yml**:

```yaml
tasks:
  app:
    description: Run app using Compose
    docker_compose: true
    docker_compose_file: docker-compose.yml
    docker_compose_down: true
    shell: true
    shell_executable: sh
    run:
      container: nodeapp
      command: npm run dev
```

### Key Fields:

* `container` refers to the service name in your Compose file (`nodeapp`).
* `shell: true` and `shell_executable: sh` override default `entrypoint` behaviour and ensure your command is executed in a shell.
* `docker_compose_down: true` cleans up containers **after the task finishes**.
  If omitted or `false`, containers remain running.

```bash
./go-batect app
```

## Example: Task with Prerequisites

```yaml
tasks:
  lint:
    run:
      container: node:20
      command: npm run lint

  test:
    run:
      container: node:20
      command: npm test

  check:
    description: Lint and test the codebase
    prerequisites:
      - lint
      - test
```

```bash
./go-batect check
```
