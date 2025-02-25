/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package add

import (
	"fmt"
	"math/rand"
	"os"
	"regexp"
	"strings"
	"unicode"

	"github.com/project-flotta/flotta-dev-cli/internal/resources"
	"github.com/spf13/cobra"
	k8svalidation "k8s.io/apimachinery/pkg/util/validation"
)

const (
	defaultImage = "quay.io/project-flotta/nginx:1.21.6"
)

// workloadCmd represents the workload command
var (
	deviceID      string
	workloadImage string
	workloadName  string

	workloadCmd = &cobra.Command{
		Use:     "workload",
		Aliases: []string{"workloads"},
		Short:   "Add a new workload",
		RunE: func(cmd *cobra.Command, args []string) error {
			if workloadImage == "" {
				workloadImage = defaultImage
			}

			// extract image name and tag for workloadName and normalize to RFC 1123 format
			splitImage := strings.Split(workloadImage, "/")
			normalizedImage, error := NormalizeString(splitImage[len(splitImage)-1])
			if error != nil {
				fmt.Fprintf(cmd.OutOrStderr(), "image: %s contains invalid characters\n", workloadImage)
				return error
			}

			if len(workloadName) == 0 {
				workloadName = normalizedImage + "-" + RandomSuffix()
			}

			client, err := resources.NewClient()
			if err != nil {
				fmt.Fprintf(cmd.OutOrStderr(), "NewClient failed: %v\n", err)
				return err
			}

			device, err := resources.NewEdgeDevice(client, deviceID)
			if err != nil {
				fmt.Fprintf(cmd.OutOrStderr(), "NewEdgeDevice failed: %v\n", err)
				return err
			}

			_, err = device.Get()
			if err != nil {
				fmt.Fprintf(cmd.OutOrStderr(), "Get device '%s' failed: %v\n", deviceID, err)
				return err
			}
			workload, err := resources.NewEdgeWorkload(client)
			if err != nil {
				fmt.Fprintf(cmd.OutOrStderr(), "NewEdgeWorkload failed: %v\n", err)
				return err
			}

			_, err = workload.Create(resources.EdgeworkloadDeviceId(workloadName, deviceID, workloadImage))
			if err != nil {
				fmt.Fprintf(cmd.OutOrStderr(), "Create workload '%s' failed: %v\n", workloadName, err)
				return err
			}

			err = device.WaitForWorkloadState(workloadName, "Running")
			if err != nil {
				fmt.Fprintf(cmd.OutOrStderr(), "WaitForWorkloadState failed: %v\n", err)
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "workload '%s' was added to device '%s'\n", workloadName, deviceID)
			return nil
		},
	}
)

func init() {
	// subcommand of add
	addCmd.AddCommand(workloadCmd)

	// define command flags
	workloadCmd.Flags().StringVarP(&deviceID, "device", "d", "", "device to run the workload on")
	workloadCmd.Flags().StringVarP(&workloadName, "name", "n", "", "name of the workload to add")
	workloadCmd.Flags().StringVarP(&workloadImage, "image", "i", "", "image of the workload")

	// mark device flag as required
	err := workloadCmd.MarkFlagRequired("device")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to set flag `name` as required: %v\n", err)
		os.Exit(1)
	}
}

func RandomSuffix() string {
	var chars = []rune("abcdefghijklmnopqrstuvwxyz")
	s := make([]rune, 8)
	for i := range s {
		s[i] = chars[rand.Intn(len(chars))]
	}
	return string(s)
}

func NormalizeString(name string) (string, error) {
	if len(name) == 0 {
		return "", fmt.Errorf("the provided name is empty")
	}
	errors := k8svalidation.IsDNS1123Subdomain(name)
	if len(errors) == 0 {
		return name, nil
	}

	// convert name to lowercase and replace '.' with '-'
	name = strings.ToLower(name)
	name = strings.Replace(name, ".", "-", -1)

	// slice string based on first and last alphanumeric character
	firstLegal := strings.IndexFunc(name, func(c rune) bool { return unicode.IsLower(c) || unicode.IsDigit(c) })
	lastLegal := strings.LastIndexFunc(name, func(c rune) bool { return unicode.IsLower(c) || unicode.IsDigit(c) })

	if firstLegal < 0 {
		return "", fmt.Errorf("the name doesn't contain a legal alphanumeric character")
	}

	name = name[firstLegal : lastLegal+1]
	reg := regexp.MustCompile("[^a-z0-9-]+")
	name = reg.ReplaceAllString(name, "")

	if len(name) > k8svalidation.DNS1123SubdomainMaxLength {
		name = name[0:k8svalidation.DNS1123SubdomainMaxLength]
	}
	return name, nil
}
