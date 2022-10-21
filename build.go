package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(buildCmd)
}

var buildCmd = &cobra.Command{
	Use:   `build`,
	Short: `Builds systems for deployment`,
	Long:  `Build will build NixOS systems locally for deployment.`,
	RunE:  runBuild,
}

func runBuild(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	systems, err := inv.matchSystems(args...)
	if err != nil {
		return err
	}
	var dont struct {
		realise bool
	}
	for _, step := range strings.Split(no, ",") {
		switch step {
		case `realise`:
			dont.realise = true
		}
	}
	if dont.realise {
		err = inv.instantiate(ctx, systems...)
	} else {
		err = inv.build(ctx, systems...)
	}
	if err != nil {
		return err
	}
	results := make(map[string]string, len(systems))
	for _, system := range systems {
		results[system] = inv.Systems[system].Result
	}
	return json.NewEncoder(os.Stdout).Encode(inv)
}

// build builds systems for a specific target, such as "system" for a NixOS system or "vhd" for a disk image.
func (inv *Inventory) build(ctx context.Context, systems ...string) error {
	for _, system := range systems {
		err := inv.Systems[system].build(ctx, system)
		if err != nil {
			return fmt.Errorf(`%w while building %q`, err, system)
		}
	}
	return nil
}

// equivalent to build that does not realise generated derivations
func (inv *Inventory) instantiate(ctx context.Context, systems ...string) error {
	systemNamesByPath := make(map[string][]string)
	for _, systemName := range systems {
		if inv.Systems[systemName].Result != `` {
			continue // already built
		}

		pathsStr := strings.Join(inv.systemPaths(systemName), "\x00")
		systemNamesByPath[pathsStr] = append(systemNamesByPath[pathsStr], systemName)
	}

	for pathsStr, systemList := range systemNamesByPath {
		args := make([]string, 0, 64)
		paths := strings.Split(pathsStr, "\x00")
		systemListJson, err := json.Marshal(systemList)
		if err != nil {
			return err
		}
		args = append(args, `--include`, `deployment=`+deploymentPath)
		for _, path := range paths {
			args = append(args, `--include`, path)
		}
		args = append(args, `--argstr`, `hiveSystemListJson`, string(systemListJson))
		args = append(args, `--expr`, `{hiveSystemListJson}: let hiveExpr = import <hive/build.nix>; targets = builtins.fromJSON hiveSystemListJson; in map (name: hiveExpr { inherit name; }) targets`)
		derivationFilenameListBytes, err := eval(ctx, `nix-instantiate`, args...)
		if err != nil {
			return err
		}
		derivationFilenames := strings.Split(string(bytes.Trim(derivationFilenameListBytes, "\n")), "\n")
		if len(derivationFilenames) != len(systemList) {
			return fmt.Errorf(
				`attempting to process a batch of %d systems (%#v), but %d results were returned: %#v`,
				len(systemList), systemList,
				len(derivationFilenames), derivationFilenames,
			)
		}

		// generate a map from derivation filenames back to system names
		systemNameByDrv := make(map[string]string, len(systemList))
		for n := range derivationFilenames {
			systemNameByDrv[derivationFilenames[n]] = systemList[n]
		}

		derivationJson, err := eval(ctx, `nix`, append([]string{`show-derivation`}, derivationFilenames...)...)
		if err != nil {
			return err
		}

		resultDerivation := make(map[string]struct {
			Outputs struct {
				Out struct {
					Path string `json:"path"`
				} `json:"out"`
			} `json:"outputs"`
		})

		if err := json.Unmarshal(derivationJson, &resultDerivation); err != nil {
			return err
		}

		if len(resultDerivation) != len(derivationFilenames) {
			return fmt.Errorf(`asked Nix to parse %d derivations, but received %d results`, len(derivationFilenames), len(resultDerivation))
		}
		for drvPath, drvStruct := range resultDerivation {
			systemName, ok := systemNameByDrv[drvPath]
			if !ok {
				return fmt.Errorf(`received information on unexpected derivation %+v`, drvPath)
			}
			cfg, ok := inv.Systems[systemName]
			if !ok {
				return fmt.Errorf(`cannot find system %+v in inventory`, systemName)
			}
			cfg.ResultDrv = drvPath
			cfg.Result = drvStruct.Outputs.Out.Path
		}
	}

	return nil
}

func (cfg *System) build(ctx context.Context, system string) error {
	if cfg.Result != `` {
		return nil // already built.
	}
	inform(ctx, `building %q`, system)
	link := filepath.Join(tmp, `system-`+system)
	args := make([]string, 0, 64)
	args = append(args, inv.Nix.Build.Flags...)
	args = append(args, `--out-link`, link, `--include`, `deployment=`+deploymentPath)
	paths := inv.systemPaths(system)
	for _, path := range paths {
		args = append(args, `--include`, path)
	}
	args = append(args, `--argstr`, `name`, system)
	args = append(args, `--expr`, `(import <hive/build.nix>)`)
	_, err := eval(ctx, `nix-build`, args...)
	if err != nil {
		return err
	}
	result, err := os.Readlink(link)
	if err != nil {
		return err
	}
	cfg.Result = result
	derivationFilename, err := eval(ctx, `nix-store`, `--query`, `--deriver`, result)
	if err == nil {
		cfg.ResultDrv = strings.TrimRight(string(derivationFilename), "\n")
	}
	return err
}
