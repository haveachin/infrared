package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	licenseCmd = &cobra.Command{
		Use:   "license",
		Short: "Shows the license of the program",
		RunE: func(_ *cobra.Command, _ []string) error {
			bb, err := files.ReadFile("LICENSE_NOTICES")
			if err != nil {
				return err
			}

			fmt.Println(string(bb))
			return nil
		},
	}
)
