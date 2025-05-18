package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Containers map[string]Container `yaml:"containers"`
	Tasks      map[string]Task      `yaml:"tasks"`
}

type Container struct {
	Image            string   `yaml:"image,omitempty"`
	Build            string   `yaml:"build,omitempty"`
	Volumes          []Volume `yaml:"volumes"`
	WorkingDirectory string   `yaml:"working_directory"`
	DockerCompose    bool     `yaml:"docker_compose,omitempty"`
}

type Volume struct {
	Local     string `yaml:"local"`
	Container string `yaml:"container"`
}

type Task struct {
	Description   string   `yaml:"description,omitempty"`
	Prerequisites []string `yaml:"prerequisites,omitempty"`
	Run           *Run     `yaml:"run,omitempty"`
	DockerCompose bool     `yaml:"docker_compose,omitempty"`
}

type Run struct {
	Container string `yaml:"container"`
	Command   string `yaml:"command"`
}

var visited = map[string]bool{}

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
				fmt.Println("Missing path after --file/-f")
				os.Exit(1)
			}
		default:
			taskArg = args[i]
		}
	}

	config, err := loadConfigWithFallback(configPath, []string{"config.yml", "batect.yml"})
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	if taskArg == "" {
		fmt.Println("Usage: mybatect [--file|-f <path>] <task-name>|--list")
		os.Exit(1)
	}

	if taskArg == "--list" {
		listTasks(config)
		return
	}

	if _, ok := config.Tasks[taskArg]; !ok {
		log.Fatalf("Task '%s' not found", taskArg)
	}

	if err := runTask(config, taskArg); err != nil {
		log.Fatalf("Error running task '%s': %v", taskArg, err)
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
		return nil, fmt.Errorf("specified config file '%s' not found", explicit)
	}

	for _, path := range fallbacks {
		if _, err := os.Stat(path); err == nil {
			return loadConfig(path)
		}
	}

	return nil, fmt.Errorf("no config file found (tried %v)", fallbacks)
}

func listTasks(cfg *Config) {
	fmt.Println("Available tasks:")
	for name, task := range cfg.Tasks {
		fmt.Printf("- %s: %s\n", name, task.Description)
	}
}

func runTask(cfg *Config, name string) error {
	if visited[name] {
		return nil
	}

	task := cfg.Tasks[name]

	for _, pre := range task.Prerequisites {
		if err := runTask(cfg, pre); err != nil {
			return err
		}
	}

	if task.DockerCompose {
		fmt.Println("⚙️  Running docker compose...")
		if err := exec.Command("docker", "compose", "up", "-d").Run(); err != nil {
			return fmt.Errorf("docker compose up failed: %w", err)
		}
		defer exec.Command("docker", "compose", "down").Run()
	}

	if task.Run != nil {
		fmt.Printf("🔧 Running task: %s\n", name)
		return runCommand(cfg, task.Run)
	}

	visited[name] = true
	return nil
}

func runCommand(cfg *Config, run *Run) error {
	container, ok := cfg.Containers[run.Container]
	if !ok {
		return fmt.Errorf("container '%s' not defined", run.Container)
	}

	image := container.Image
	if container.Build != "" {
		image = "go-batect_" + run.Container
		buildCmd := exec.Command("docker", "build", "-t", image, container.Build)
		buildCmd.Stdout = os.Stdout
		buildCmd.Stderr = os.Stderr
		fmt.Printf("🏗️  Building image '%s'...\n", image)
		if err := buildCmd.Run(); err != nil {
			return fmt.Errorf("docker build failed: %w", err)
		}
	}

	args := []string{"run", "--rm"}
	for _, vol := range container.Volumes {
		absLocal, err := filepath.Abs(vol.Local)
		if err != nil {
			return err
		}
		args = append(args, "-v", fmt.Sprintf("%s:%s", absLocal, vol.Container))
	}

	if container.WorkingDirectory != "" {
		args = append(args, "-w", container.WorkingDirectory)
	}

	args = append(args, image)
	args = append(args, strings.Fields(run.Command)...)

	cmd := exec.Command("docker", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	fmt.Printf("🚀 docker %s\n", strings.Join(args, " "))
	return cmd.Run()
}
