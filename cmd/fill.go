package cmd

import (
	"embed"
	"errors"
	"github.com/Azure/draft/pkg/osutil"
	"github.com/Azure/draft/pkg/templatewriter/writers"
	"github.com/Azure/draft/template"
	"path"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

const (
	TYPE_STRING = "string"
	TYPE_INT    = "int"
	TYPE_BOOL   = "bool"
)

var baseFillCmd = &cobra.Command{
	Use:   "fill",
	Short: "fills a template based on input parameters",
	Long:  "This command will fill the specified template with the input parameters.",
}

type FillCMD struct {
	Directory        string
	FillCommand      *FillCommandDefinition       `yaml:"fillcmd"`
	FillVariables    []*FillVariableDefinition    `yaml:"variables"`
	VariableDefaults []*VariableDefaultDefinition `yaml:"variableDefaults"`
	VariableInput    map[string]*string
}

type FillCommandDefinition struct {
	Use   string `yaml:"use"`
	Short string `yaml:"short"`
	Long  string `yaml:"long"`
}

type FillVariableDefinition struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Type        string `yaml:"type"`
}

type VariableDefaultDefinition struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Type        string `yaml:"type"`
	Value       string `yaml:"value"`
}

func (fillcmd *FillCMD) run() error {
	templateWriter := &writers.LocalFSWriter{}
	newOverrideVars := map[string]string{}
	for k, v := range fillcmd.VariableInput {
		if v != nil {
			newOverrideVars[k] = *v
		}
	}

	// TODO: Destination input flag
	err := osutil.CopyDir(&template.Templates, fillcmd.Directory, "./", nil, newOverrideVars, templateWriter)
	if err != nil {
		return err
	}

	return nil
}

func init() {
	rootCmd.AddCommand(newFillCommand())
}

func newFillCommand() *cobra.Command {

	draftYamls, err := getAllDraftYamls(&template.Templates, "")
	if err != nil {
		log.Fatalf("failed to build draft fill commands: %s", err.Error())
	}

	var fillCommands []*cobra.Command
	for _, draftYamlPath := range draftYamls {
		fillCommand, err := parseFileToFillCommand(&template.Templates, draftYamlPath)
		if err != nil {
			log.Fatalf("failed to parse file %s to FillCMD: %s", draftYamlPath, err.Error())
		}

		if fillCommand.FillCommand == nil {
			continue
		}

		cobraFillCmd, err := createCobraCommandFromDefinition(fillCommand)
		if err != nil {
			log.Fatalf("failed to create cobra command from file %s: %s", draftYamlPath, err.Error())
		}

		fillCommands = append(fillCommands, cobraFillCmd)
	}

	baseFillCmd.AddCommand(fillCommands...)

	return baseFillCmd
}

func getAllDraftYamls(fs *embed.FS, dir string) (out []string, err error) {
	if len(dir) == 0 {
		dir = "."
	}

	entries, err := fs.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		fp := path.Join(dir, entry.Name())
		if entry.IsDir() {
			res, err := getAllDraftYamls(fs, fp)
			if err != nil {
				return nil, err
			}

			out = append(out, res...)

			continue
		}

		if strings.EqualFold(entry.Name(), "draft.yaml") {
			out = append(out, fp)
		}
	}

	return
}

func parseFileToFillCommand(fs *embed.FS, path string) (*FillCMD, error) {
	yamlBytes, err := fs.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var fillCMD FillCMD
	err = yaml.Unmarshal(yamlBytes, &fillCMD)
	if err != nil {
		return nil, err
	}

	fillCMD.Directory = strings.ReplaceAll(path, "/draft.yaml", "")
	return &fillCMD, nil
}

func createCobraCommandFromDefinition(fillCMD *FillCMD) (*cobra.Command, error) {
	if fillCMD == nil || fillCMD.FillCommand == nil {
		return nil, errors.New("failed to create cobra command from nil fillCMD definition")
	}

	cmd := &cobra.Command{
		Use:   fillCMD.FillCommand.Use,
		Short: fillCMD.FillCommand.Short,
		Long:  fillCMD.FillCommand.Long,
		RunE: func(cmd *cobra.Command, args []string) error {
			return fillCMD.run()
		},
	}

	f := cmd.Flags()
	fillCMD.VariableInput = make(map[string]*string)
	for _, cmdVar := range fillCMD.FillVariables {
		fillCMD.VariableInput[cmdVar.Name] = f.String(strings.ToLower(cmdVar.Name), emptyDefaultFlagValue, cmdVar.Description)
	}

	for _, cmdVar := range fillCMD.VariableDefaults {
		fillCMD.VariableInput[cmdVar.Name] = f.String(strings.ToLower(cmdVar.Name), cmdVar.Value, cmdVar.Description)
	}

	return cmd, nil
}
