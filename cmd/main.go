package main

import (
	"fmt"
	"os"

	"github.com/urfave/cli/v2"

	"github.com/cocov-ci/worker/commands"
)

func envs(base string) []string {
	return []string{"COCOV_" + base, base}
}

func main() {
	fmt.Println("             .                     ")
	fmt.Println("             *                     ")
	fmt.Println("            :*                     ")
	fmt.Println("           ::@=-:  *= :            ")
	fmt.Println("          -* #--+@+%##@- :         ")
	fmt.Println("          +%-:#  .+*-*.=#*         ")
	fmt.Println("          .%+.=:    :  . #%-       ")
	fmt.Println("            -*:           #:       Cocov Worker")
	fmt.Println("                -:  +.  .=.        Copyright (c) 2022-2023 - The Cocov Authors")
	fmt.Println("             .**=       %%#=       Licensed under GPL-3.0")
	fmt.Println("            +*. =:   :-=+..        ")
	fmt.Println("          .#=    =#=-=%::*         ")
	fmt.Println("         -%:      =#. :+*=         ")
	fmt.Println("        +#         ##.  .          ")
	fmt.Println("       #*          :+@.            ")
	fmt.Println("     .%+          :% @+            ")
	fmt.Println("    .%=           -@:@*            ")

	app := cli.NewApp()
	app.Name = "cocov-worker"
	app.Usage = "Executes Cocov checks on commits"
	app.Version = "0.1"
	app.DefaultCommand = "run"
	app.Flags = []cli.Flag{
		&cli.StringFlag{Name: "redis-url", EnvVars: envs("REDIS_URL"), Required: true},
		&cli.StringFlag{Name: "docker-socket", EnvVars: envs("DOCKER_SOCKET"), Required: true},
		&cli.IntFlag{Name: "max-parallel-jobs", EnvVars: envs("MAX_PARALLEL_JOBS"), Required: false, Value: 5},
		&cli.StringFlag{Name: "api-url", EnvVars: envs("API_URL"), Required: true},
		&cli.StringFlag{Name: "service-token", EnvVars: envs("SERVICE_TOKEN"), Required: true},
		&cli.StringFlag{Name: "gs-storage-mode", EnvVars: envs("GIT_SERVICE_STORAGE_MODE"), Required: true},
		&cli.StringFlag{Name: "gs-local-storage-path", EnvVars: envs("GIT_SERVICE_LOCAL_STORAGE_PATH"), Required: false},
		&cli.StringFlag{Name: "gs-s3-bucket-name", EnvVars: envs("GIT_SERVICE_S3_BUCKET_NAME"), Required: false},
	}
	app.Authors = []*cli.Author{
		{Name: "Victor \"Vito\" Gama", Email: "hey@vito.io"},
	}
	app.Copyright = "Copyright (c) 2022-2023 - The Cocov Authors"
	app.Commands = []*cli.Command{
		{
			Name:   "run",
			Usage:  "Starts a worker",
			Action: commands.Run,
		},
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Println()
		fmt.Printf("Error: %s\n", err)
		os.Exit(1)
	}
}
