Feature: go-batect task execution
  As a developer
  I want to run tasks in isolated Docker containers
  So that my development environment is consistent

  Scenario: List available tasks
    Given a configuration file "batect.yml" with:
      """
      tasks:
        hello:
          description: "Prints hello"
          run:
            container: bash
            command: echo hello
      containers:
        bash:
          image: bash:latest
      """
    When I run "go-batect --list"
    Then the output should contain "hello: Prints hello"

  Scenario: Run a simple task
    Given a configuration file "batect.yml" with:
      """
      tasks:
        hello:
          description: "Prints hello"
          run:
            container: bash
            command: echo "Hello World"
      containers:
        bash:
          image: bash:latest
      """
    When I run "go-batect hello"
    Then the output should contain "Hello World"

  Scenario: Run a task with prerequisites
    Given a configuration file "batect.yml" with:
      """
      tasks:
        first:
          description: "First task"
          run:
            container: bash
            command: echo "First"
        second:
          description: "Second task"
          prerequisites:
            - first
          run:
            container: bash
            command: echo "Second"
      containers:
        bash:
          image: bash:latest
      """
    When I run "go-batect second"
    Then the output should contain "🔧 Running task: first"
    And the output should contain "First"
    And the output should contain "🔧 Running task: second"
    And the output should contain "Second"

  Scenario: Run a task with a healthcheck
    Given a configuration file "batect.yml" with:
      """
      tasks:
        health:
          description: "Healthcheck task"
          run:
            container: healthy-bash
            command: echo "Healthy"
      containers:
        healthy-bash:
          image: bash:latest
          healthcheck:
            command: "ls / || exit 1"
            interval: 1s
            retries: 2
      """
    When I run "go-batect health"
    Then the output should contain "Healthy"

  Scenario: Run a task with docker-compose
    Given a configuration file "batect.yml" with:
      """
      tasks:
        app:
          description: "Run app with compose"
          docker_compose: true
          docker_compose_file: node-docker-compose.yml
          run:
            container: nodeapp
            command: echo "Compose Success"
      containers:
        nodeapp:
          image: bash:latest
      """
    And a file "node-docker-compose.yml" with:
      """
      services:
        nodeapp:
          image: bash:latest
          command: sleep 1000
      """
    When I run "go-batect app"
    Then the output should contain "Compose Success"

  Scenario: Run a task with legacy build forced
    Given a configuration file "batect.yml" with:
      """
      tasks:
        legacy:
          description: "Forced legacy build"
          run:
            container: legacy-container
            command: echo "Legacy Success"
      containers:
        legacy-container:
          build: ./docker-legacy
          legacy_build: true
      """
    And a file "docker-legacy/Dockerfile" with:
      """
      FROM bash:latest
      ENTRYPOINT ["sh", "-c"]
      """
    When I run "go-batect legacy"
    Then the output should contain "Forcing legacy docker build for container 'legacy-container'"
    And the output should contain "Legacy Success"
