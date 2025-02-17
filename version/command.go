package version

import (
	"encoding/json"
	"strings"

	"github.com/line/ostracon/libs/cli"
	"github.com/spf13/cobra"
	yaml "gopkg.in/yaml.v2"
)

const flagLong = "long"

// NewVersionCommand returns a CLI command to interactively print the application binary version information.
func NewVersionCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print the application binary version information",
		RunE: func(cmd *cobra.Command, _ []string) error {
			verInfo := NewInfo()

			if long, _ := cmd.Flags().GetBool(flagLong); !long {
				cmd.Println(verInfo.Version)
				return nil
			}

			var (
				bz  []byte
				err error
			)

			output, _ := cmd.Flags().GetString(cli.OutputFlag)
			switch strings.ToLower(output) {
			case "json":
				bz, err = json.Marshal(verInfo)

			default:
				bz, err = yaml.Marshal(&verInfo)
			}

			if err != nil {
				return err
			}

			cmd.Println(string(bz))
			return nil
		},
	}

	cmd.Flags().Bool(flagLong, false, "Print long version information")
	cmd.Flags().StringP(cli.OutputFlag, "o", "text", "Output format (text|json)")

	return cmd
}
