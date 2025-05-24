package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Containers map[string]Container `yaml:"containers"`
	Tasks      map[string]Task      `yaml:"tasks"`
}

type Container struct {
	Image            string       `yaml:"image,omitempty"`
	Build            string       `yaml:"build,omitempty"`
	LegacyBuild      bool         `yaml:"legacy_build,omitempty"`
	Volumes          []Volume     `yaml:"volumes"`
	WorkingDirectory string       `yaml:"working_directory"`
	DockerCompose    bool         `yaml:"docker_compose,omitempty"`
	HealthCheck      *HealthCheck `yaml:"healthcheck,omitempty"`
}

type HealthCheck struct {
	Command     string `yaml:"command"`
	Interval    string `yaml:"interval,omitempty"`
	Timeout     string `yaml:"timeout,omitempty"`
	Retries     int    `yaml:"retries,omitempty"`
	StartPeriod string `yaml:"start_period,omitempty"`
}

type Volume struct {
	Local     string `yaml:"local"`
	Container string `yaml:"container"`
}

type Task struct {
	Description       string `yaml:"description"`
	Shell             bool   `yaml:"shell"`
	ShellExecutable   string `yaml:"shell_executable"`
	DockerCompose     bool   `yaml:"docker_compose"`
	DockerComposeDown bool   `yaml:"docker_compose_down"`
	DockerComposeFile string `yaml:"docker_compose_file"`
	Run               struct {
		Container string `yaml:"container"`
		Command   string `yaml:"command"`
	} `yaml:"run"`
	Prerequisites []string `yaml:"prerequisites"`
}

type Run struct {
	Container string `yaml:"container"`
	Command   string `yaml:"command"`
}

func main() {
	var configPath string
	args := os.Args[1:]
	taskArg := ""

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--file", "-f":
			if i+1 < len(args) {
				configPath = args[i+1]
				i++
			} else {
				fmt.Println("❗ Missing path after --file/-f")
				os.Exit(1)
			}
		default:
			taskArg = args[i]
		}
	}

	config, err := loadConfigWithFallback(configPath, []string{"config.yml", "batect.yml"})
	if err != nil {
		log.Fatalf("❗ Failed to load config: %v", err)
	}

	if taskArg == "" {
		fmt.Println("❗ No task given. Usage: go-batect [--file|-f <path>] <task-name>")
		os.Exit(1)
	}

	if taskArg == "--list" {
		listTasks(config)
		return
	}

	if _, ok := config.Tasks[taskArg]; !ok {
		log.Fatalf("❗ Task '%s' not found", taskArg)
	}

	if err := runTask(config, taskArg); err != nil {
		log.Fatalf("❗ Error running task '%s': %v", taskArg, err)
	}
}

func loadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	err = yaml.Unmarshal(data, &cfg)
	return &cfg, err
}

func loadConfigWithFallback(explicit string, fallbacks []string) (*Config, error) {
	if explicit != "" {
		if _, err := os.Stat(explicit); err == nil {
			return loadConfig(explicit)
		}
		return nil, fmt.Errorf("❗ specified config file '%s' not found", explicit)
	}

	for _, path := range fallbacks {
		if _, err := os.Stat(path); err == nil {
			return loadConfig(path)
		}
	}

	return nil, fmt.Errorf("❗ no config file found (tried %v)", fallbacks)
}

func listTasks(cfg *Config) {
	fmt.Println("Available tasks:")
	for name, task := range cfg.Tasks {
		fmt.Printf("- %s: %s\n", name, task.Description)
	}
}

func runTask(config *Config, name string) error {
	task, ok := config.Tasks[name]
	if !ok {
		return fmt.Errorf("❗ task '%s' not found", name)
	}

	hasRun := task.Run.Container != ""

	if !hasRun {
		log.Printf("🔧 Running task: %s", name)
	}

	for _, prereq := range task.Prerequisites {
		if err := runTask(config, prereq); err != nil {
			return fmt.Errorf("❗ failed prerequisite '%s': %w", prereq, err)
		}
	}

	if hasRun {
		log.Printf("🔧 Running task: %s", name)
	}

	if task.DockerCompose {
		fmt.Println("⚙️ Running docker-compose...")

		upCmd := exec.Command("docker", "compose", "-f", task.DockerComposeFile, "up", "-d")
		upCmd.Stdout = os.Stdout
		upCmd.Stderr = os.Stderr
		if err := upCmd.Run(); err != nil {
			return fmt.Errorf("❗ docker compose up failed: %w", err)
		}

		if container, ok := config.Containers[task.Run.Container]; ok && container.HealthCheck != nil {
			fmt.Printf("⏳ Waiting for container '%s' to be healthy...\n", task.Run.Container)
			if err := waitForHealthy(task.DockerComposeFile, task.Run.Container); err != nil {
				return fmt.Errorf("❗ container '%s' failed health check: %w", task.Run.Container, err)
			}
		}

		if task.DockerComposeDown {
			defer func() {
				downCmd := exec.Command("docker", "compose", "-f", task.DockerComposeFile, "down")
				downCmd.Stdout = os.Stdout
				downCmd.Stderr = os.Stderr
				_ = downCmd.Run()
			}()
		}

		execArgs := []string{"exec", task.Run.Container}
		if task.Shell {
			shell := task.ShellExecutable
			if shell == "" {
				shell = "sh"
			}
			execArgs = append(execArgs, shell, "-c", task.Run.Command)
		} else {
			execArgs = append(execArgs, strings.Fields(task.Run.Command)...)
		}

		fmt.Printf("🚀 docker compose %s\n", strings.Join(execArgs, " "))
		execCmd := exec.Command("docker", append([]string{"compose", "-f", task.DockerComposeFile}, execArgs...)...)
		execCmd.Stdout = os.Stdout
		execCmd.Stderr = os.Stderr
		execCmd.Stdin = os.Stdin

		if err := execCmd.Run(); err != nil {
			return fmt.Errorf("❗ error running task '%s': %w", name, err)
		}

		return nil
	}

	run := task.Run
	container, ok := config.Containers[run.Container]
	if !ok && len(task.Prerequisites) == 0 && run.Container == "" {
		return fmt.Errorf("❗ container '%s' not found for task '%s'", run.Container, name)
	}

	image := container.Image

	if container.Build != "" {
		image = "go-batect_" + run.Container
		fmt.Printf("🏗️  Building image '%s'...\n", image)

		if container.LegacyBuild {
			log.Printf("ℹ️  Forcing legacy docker build for container '%s'", run.Container)
			buildCmd := exec.Command("docker", "build", "-t", image, container.Build)
			buildCmd.Stdout = os.Stdout
			buildCmd.Stderr = os.Stderr
			if err := buildCmd.Run(); err != nil {
				return fmt.Errorf("❗ docker build failed: %w", err)
			}
		} else {
			buildxCmd := exec.Command("docker", "buildx", "build", "-t", image, container.Build, "--load")
			buildxCmd.Stdout = os.Stdout
			buildxCmd.Stderr = os.Stderr
			if err := buildxCmd.Run(); err != nil {
				log.Printf("⚠️  docker buildx failed, falling back to legacy docker build: %v", err)
				buildCmd := exec.Command("docker", "build", "-t", image, container.Build)
				buildCmd.Stdout = os.Stdout
				buildCmd.Stderr = os.Stderr
				if err := buildCmd.Run(); err != nil {
					return fmt.Errorf("❗ docker build failed: %w", err)
				}
			}
		}
	}

	args := []string{"run", "--rm"}

	for _, vol := range container.Volumes {
		absLocal, err := filepath.Abs(vol.Local)
		if err != nil {
			return fmt.Errorf("❗ invalid volume path '%s': %w", vol.Local, err)
		}
		args = append(args, "-v", fmt.Sprintf("%s:%s", absLocal, vol.Container))
	}

	if container.WorkingDirectory != "" {
		args = append(args, "-w", container.WorkingDirectory)
	}

	if container.HealthCheck != nil {
		if container.HealthCheck.Command != "" {
			args = append(args, "--health-cmd", container.HealthCheck.Command)
		}
		if container.HealthCheck.Interval != "" {
			args = append(args, "--health-interval", container.HealthCheck.Interval)
		}
		if container.HealthCheck.Timeout != "" {
			args = append(args, "--health-timeout", container.HealthCheck.Timeout)
		}
		if container.HealthCheck.Retries > 0 {
			args = append(args, "--health-retries", fmt.Sprintf("%d", container.HealthCheck.Retries))
		}
		if container.HealthCheck.StartPeriod != "" {
			args = append(args, "--health-start-period", container.HealthCheck.StartPeriod)
		}
	}

	useShell := task.Shell
	if useShell {
		shell := task.ShellExecutable
		if shell == "" {
			shell = "sh"
		}
		args = append(args, "--entrypoint", shell)
	}

	args = append(args, image)

	if useShell {
		args = append(args, "-c", run.Command)
	} else {
		args = append(args, strings.Fields(run.Command)...)
	}

	if len(task.Prerequisites) == 0 && task.Run.Container == "" {
		return fmt.Errorf("❗ task '%s' has no prerequisites or run command defined", name)
	}

	if task.Run.Container != "" {
		log.Printf("🚀 docker %s", strings.Join(args, " "))

		cmd := exec.Command("docker", args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin

		if err := cmd.Run(); err != nil {
			return fmt.Errorf("❗ error running task '%s': %w", name, err)
		}
	}

	return nil
}

func waitForHealthy(composeFile, containerName string) error {
	timeout := time.After(5 * time.Minute)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			fmt.Printf("❌ Timeout reached waiting for '%s' to become healthy. Fetching logs...\n", containerName)
			showLogs(composeFile, containerName)
			return fmt.Errorf("timeout waiting for healthcheck")
		case <-ticker.C:
			cmd := exec.Command("docker", "compose", "-f", composeFile, "ps", containerName, "--format", "json")
			output, err := cmd.Output()
			if err != nil {
				return err
			}

			outStr := string(output)
			if strings.Contains(outStr, "\"Health\":\"healthy\"") || strings.Contains(outStr, "healthy") {
				return nil
			}

			if strings.Contains(outStr, "\"Health\":\"unhealthy\"") || strings.Contains(outStr, "unhealthy") {
				fmt.Printf("❌ Container '%s' is unhealthy. Fetching logs...\n", containerName)
				showLogs(composeFile, containerName)
				return fmt.Errorf("container is unhealthy")
			}
		}
	}
}

func showLogs(composeFile, containerName string) {
	fmt.Printf("📝 Last 20 lines of logs for '%s':\n", containerName)
	cmd := exec.Command("docker", "compose", "-f", composeFile, "logs", "--tail", "20", containerName)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	_ = cmd.Run()
}
