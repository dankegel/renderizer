package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/imdario/mergo"
	"github.com/kardianos/osext"
	"github.com/urfave/cli"
	"gopkg.in/yaml.v2"
)

var (
	version  = "2.0.0"
	commit   = "unknown"
	date     = "20060102T150405"
	selfn, _ = osext.Executable()
	selfz    = filepath.Base(selfn)
	semver   = version + "-" + date + "." + commit[:7]
	appver   = selfz + "/" + semver
)

//
type Settings struct {
	// Capitalization is a positional toggles. The following variable names are capitalized (title-case).
	Capitalize bool
	// Set the Missing Key template option. Defaults to "error".
	MissingKey string
	// Configuration yaml
	ConfigFiles cli.StringSlice
	Defaulted   bool
	Config      map[string]interface{}
	//
	Arguments []string
	// Add the environment map to the variables.
	Environment string
	//
	OutputExtension string
	//
	TimeFormat string
	//
	Stdin bool
	//
	Debugging bool
	//
	Verbose bool
}

//
var settings = Settings{
	Capitalize:  true,
	MissingKey:  "error",
	TimeFormat:  "20060102T150405",
	Environment: "env",
	Config:      map[string]interface{}{},
	ConfigFiles: []string{},
	Arguments:   []string{},
}

//
func main() {
	app := cli.NewApp()
	app.Name = "renderizer"
	app.Usage = "renderizer [options] [--name=value...] template..."
	app.UsageText = "Template renderer"
	app.Version = appver
	app.EnableBashCompletion = true

	configs := cli.StringSlice{}

	app.Commands = []cli.Command{
		{
			Name:  "version",
			Usage: "Shows the app version",
			Action: func(ctx *cli.Context) error {
				fmt.Println(ctx.App.Version)
				return nil
			},
		},
	}

	app.Flags = []cli.Flag{
		cli.StringSliceFlag{
			Name:   "settings, S, s",
			Usage:  `load the settings from the provided YAMLs (default: ".renderizer.yaml")`,
			Value:  &configs,
			EnvVar: "RENDERIZER",
		},
		cli.StringFlag{
			Name:        "missing, M, m",
			Usage:       "the 'missingkey' template option (default|zero|error)",
			Value:       "error",
			EnvVar:      "RENDERIZER_MISSINGKEY",
			Destination: &settings.MissingKey,
		},
		cli.StringFlag{
			Name:   "environment, env, E, e",
			Usage:  "load the environment into the variable name instead of as 'env'",
			Value:  settings.Environment,
			EnvVar: "RENDERIZER_ENVIRONMENT",
		},
		cli.BoolFlag{
			Name:        "stdin, c",
			Usage:       "read from stdin",
			Destination: &settings.Stdin,
		},
		cli.BoolFlag{
			Name:        "debugging, debug, D",
			Usage:       "enable debugging server",
			Destination: &settings.Debugging,
		},
		cli.BoolFlag{
			Name:        "verbose, V",
			Usage:       "enable verbose output",
			Destination: &settings.Verbose,
		},
	}

	app.Before = func(ctx *cli.Context) error {

		fi, _ := os.Stdin.Stat()

		settings.Stdin = settings.Stdin || (fi.Mode()&os.ModeCharDevice) == 0

		settings.Arguments = append(settings.Arguments, ctx.Args()...)

		mainName, mainTemplate, noTemplate := "", "", true
		if largs := len(settings.Arguments); largs > 0 {
			i, found := largs-1, -1
			for ; i > 0; i-- {
				if !strings.HasPrefix(settings.Arguments[i], "--") {
					found = i
				}
			}
			if found >= 0 {
				noTemplate = false
				mainTemplate = settings.Arguments[found]
			}
		}

		if mainTemplate == "" && !settings.Stdin {
			// Try default the template name
			folderName, err := os.Getwd()
			if err != nil {
				log.Println(err)
				folderName = "renderizer"
			} else {
				folderName = filepath.Base(folderName)
			}

			for _, ext := range []string{".tmpl", ""} {
				for _, try := range []string{"yaml", "json", "html", "txt", "xml", ""} {
					name := fmt.Sprintf("%s.%s%s", folderName, try, ext)
					if _, err := os.Stat(name); err == nil {
						if settings.Verbose {
							log.Printf("using template: %+v", name)
						}
						mainTemplate = name
					}
				}
			}
		}

		if noTemplate {
			settings.Arguments = append(settings.Arguments, mainTemplate)
		}
		mainName = strings.Split(strings.TrimLeft(filepath.Base(mainTemplate), "."), ".")[0]

		switch settings.MissingKey {
		case "zero", "error", "default", "invalid":
		default:
			fmt.Fprintf(os.Stderr, "ERROR: Resetting invalid missingkey: %+v", settings.MissingKey)
			settings.MissingKey = "error"
		}

		if len(configs) == 0 {
			settings.Defaulted = true
			settings.ConfigFiles = []string{"." + mainName + ".yaml"}
		} else {
			settings.ConfigFiles = configs
		}

		for _, config := range settings.ConfigFiles {
			in, err := ioutil.ReadFile(config)
			if err != nil {
				if !settings.Defaulted {
					return err
				}
			} else {
				loaded := map[string]interface{}{}
				err := yaml.Unmarshal(in, &loaded)
				if err != nil {
					return err
				}
				if settings.Verbose || settings.Defaulted {
					log.Printf("using settings: %+v", settings.ConfigFiles)
				}
				loaded = retyper(loaded)
				if settings.Debugging {
					log.Printf("loaded: %s = %#v", config, loaded)
				} else if settings.Verbose {
					log.Printf("loaded: %s = %+v", config, loaded)
				}
				mergo.Merge(&settings.Config, loaded)
			}
		}

		if settings.Debugging {
			log.Printf("--settings:%#v", settings)
		} else if settings.Verbose {
			log.Printf("--settings:%+v", settings)
		}

		return nil
	}

	app.Action = renderizer
	app.Run(os.Args)
}
