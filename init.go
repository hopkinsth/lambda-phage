package main

import "github.com/hopkinsth/lambda-phage/Godeps/_workspace/src/github.com/spf13/cobra"
import "github.com/hopkinsth/lambda-phage/Godeps/_workspace/src/github.com/aws/aws-sdk-go/service/iam"
import "github.com/hopkinsth/lambda-phage/Godeps/_workspace/src/github.com/aws/aws-sdk-go/aws"
import "github.com/hopkinsth/lambda-phage/Godeps/_workspace/src/github.com/tj/go-debug"
import "strings"
import "fmt"
import "os"

func init() {
	initCmd := &cobra.Command{
		Use:   "init",
		Short: "initializes a config for your function",
		Run:   initPhage,
	}

	cmds = append(cmds, initCmd)
}

// helps you build a config file
func initPhage(c *cobra.Command, _ []string) {
	var err error
	fmt.Println(`
  HELLO AND WELCOME
  
  This command will help you set up your code for deployment to Lambda!
  
  But we need some information from you, like what you want to name
  your function and a few other things!
  
  Please answer the prompts as they appear below:
	`)

	iCfg := new(Config)
	iCfg.IamRole = new(IamRole)
	iCfg.Location = new(Location)
	prompts := getPrompts(iCfg)

	err = prompts.run()

	// merge in any existing properties from the config object
	var wCfg *Config
	if cfg != nil {
		// if there's a config object, merge these two together
		cfg.merge(iCfg)
		wCfg = cfg
	} else {
		wCfg = iCfg
	}

	cfgFile, _ := c.Flags().GetString("config")
	err = wCfg.writeToFile(cfgFile)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Setup complete; saved config to %s\n", cfgFile)
}

// returns all the prompts needed for the `init` command
func getPrompts(cfg *Config) prompter {
	wd, _ := os.Getwd()
	st, _ := os.Stat(wd)

	iamRoles, roleMap := getIamRoles()
	ps := new(promptSet)

	return ps.add().
		withString(&cfg.Name).
		isRequired().
		setText("Enter a project name").
		setDef(st.Name()).
		done().
		add().
		withString(&cfg.Description).
		setText("Enter a project description if you'd like").
		setDef("").
		done().
		add().
		withString(&cfg.Archive).
		setText("Enter a archive name if you'd like").
		setDef(st.Name() + ".zip").
		done().
		add().
		withString(&cfg.Runtime).
		isRequired().
		setText("What runtime are you using: nodejs, java8, or python 2.7?").
		setDef("nodejs").
		withCompleter(
		func(l string) []string {
			// there can only be one
			r := make([]string, 1)
			if len(l) == 0 {
				r[0] = ""
			} else {
				switch string(l[0]) {
				case "n":
					r[0] = "nodejs"
				case "j":
					r[0] = "java8"
				case "p":
					r[0] = "python2.7"
				}
			}

			return r
		},
	).
		done().
		add().
		withString(&cfg.EntryPoint).
		isRequired().
		setText("Enter an entry point or handler name").
		setDef("index.handler").
		done().
		add().
		withInt(&cfg.MemorySize).
		setText("Enter memory size").
		setDef("128").
		done().
		add().
		withInt(&cfg.Timeout).
		setText("Enter timeout").
		setDef("5").
		done().
		add().
		withStringSet(&cfg.Regions).
		setText("Enter AWS regions where this function will run").
		setDef("us-east-1").
		done().
		add().
		withFunc(
		func(s string) {
			// if this looks like an ARN,
			// we'll assume it is... for now
			if strings.Index(s, "arn:aws:iam::") == 0 {
				cfg.IamRole.Arn = &s
			} else {
				// check to see if this name is inside the role map
				if arn, ok := roleMap[s]; ok {
					cfg.IamRole.Arn = arn
				} else {
					// if not in role map, set the name
					cfg.IamRole.Name = &s
				}
			}
		},
	).
		isRequired().
		setText("IAM Role").
		setDescription(
		"\nAll Lambda functions must run with an IAM role, which gives them access\n" +
			"to various resources in AWS. What role do you want to assign this function?\n" +
			"Type a name and we'll try to auto-complete it if you press the tab key\n",
	).
		setDef("").
		withCompleter(
		func(l string) []string {
			c := make([]string, 0)
			for _, role := range iamRoles {
				if strings.HasPrefix(*role.Name, l) {
					c = append(c, *role.Name)
				}
			}

			return c
		},
	).
		done().
		add().
		withString(&cfg.Location.S3Bucket).
		setText("s3 bucket you want to upload your code to (if any)").
		setDef("").
		done().
		add().
		withString(&cfg.Location.S3Key).
		setText("s3 folder your code should be stored inside (if any)").
		setDef("").
		done().
		add().
		withString(&cfg.Location.S3Region).
		setText("s3 region where your bucket is (if any)").
		setDef("").
		done()
}

// pulls all the IAM roles from your account
func getIamRoles() ([]*IamRole, map[string]*string) {
	debug := debug.Debug("core.getIamRoles")
	i := iam.New(nil)
	r, err := i.ListRoles(&iam.ListRolesInput{
		// try loading up to 1000 roles now
		MaxItems: aws.Int64(1000),
	})

	if err != nil {
		debug("getting IAM roles failed! maybe you don't have permission to do that?")
		return []*IamRole{}, map[string]*string{}
	}

	roles := make([]*IamRole, len(r.Roles))
	roleMap := make(map[string]*string)
	for i, r := range r.Roles {
		roles[i] = &IamRole{
			Arn:  r.Arn,
			Name: r.RoleName,
		}
		roleMap[*r.RoleName] = r.Arn
	}

	return roles, roleMap
}
