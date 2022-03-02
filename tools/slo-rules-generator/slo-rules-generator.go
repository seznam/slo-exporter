package main

import (
	"fmt"
	"os"

	"github.com/prometheus/prometheus/pkg/rulefmt"
	"gopkg.in/yaml.v3"
)


const (
	domainFileHeaderComment = `#
# This file has been generated by slo-rules-generator.
# DO NOT EDIT MANUALLY!
#
`
)

func yamlStr(s string) yaml.Node {
	n := yaml.Node{}
	n.SetString(s)
	return n
}

type Labels map[string]string

func (l Labels) Merge(with Labels) Labels {
	out := Labels{}
	for k, v := range l {
		out[k] = v
	}
	for k, v := range with {
		out[k] = v
	}
	return out
}

type SloConfiguration map[string]Domain

func main() {
	if len(os.Args) != 2 {
		fmt.Printf("Usage: %s <slo-conf-path>", os.Args[0])
		os.Exit(1)
	}
	confFilename := os.Args[1]
	data, err := os.ReadFile(confFilename)
	if err != nil {
		fmt.Printf("Unable read file '%s': %v", confFilename, err)
		os.Exit(2)
	}
	conf := SloConfiguration{}
	err = yaml.Unmarshal(data, &conf)
	if err != nil {
		fmt.Printf("Unable to parse input configuration: %s", err.Error())
		os.Exit(2)
	}
	for domainName, domainConf := range conf {
			if errs := domainConf.IsValid(); len(errs) > 0 {
				fmt.Printf("Error while validation domain '%s' configuration:\n%v", domainName, errs)
				os.Exit(2)
			}
			domainGroups := rulefmt.RuleGroups{
				Groups: domainConf.AsRuleGroups(domainName),
			}
			data, err := yaml.Marshal(domainGroups)
			if err != nil {
				fmt.Printf("Unable to marshall domain %s: %v", domainName, err)
				os.Exit(2)
			}
			fname := fmt.Sprintf("%s.yaml", domainName)
			f, err := os.Create(fname)
			if err != nil {
				fmt.Printf("Error while creating file %s: %v", fname, err)
				os.Exit(1)
			}
			fmt.Printf("-> %s\n", fname)
			defer f.Close()
			_, err = f.WriteString(domainFileHeaderComment)
			if err != nil {
				fmt.Printf("Error while writing to %s: %v", fname, err)
				os.Exit(1)
			}
			_, err = f.Write(data)
			if err != nil {
				fmt.Printf("Error while writing to %s: %v", fname, err)
				os.Exit(1)
			}
	}
}



