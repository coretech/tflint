package main

import (
	"crypto/md5"
	"encoding/hex"
	"sync"

	version "github.com/hashicorp/go-version"
	"github.com/hashicorp/hcl2/hcl"
	"github.com/hashicorp/terraform/addrs"
	"github.com/hashicorp/terraform/configs"
	"github.com/hashicorp/terraform/configs/configload"
	"github.com/hashicorp/terraform/terraform"
	"github.com/k0kubun/pp"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/gocty"
)

func main() {
	loader, err := configload.NewLoader(&configload.Config{
		ModulesDir: ".terraform/modules",
	})
	if err != nil {
		panic(err)
	}

	// configload LoadConfig()
	rootMod, diags := loader.Parser().LoadConfigDir(".")
	if diags.HasErrors() {
		panic(diags)
	}
	cfg, diags := configs.BuildConfig(rootMod, configs.ModuleWalkerFunc(
		func(req *configs.ModuleRequest) (*configs.Module, *version.Version, hcl.Diagnostics) {
			sum := md5.Sum([]byte("1." + req.Name + ";" + req.SourceAddr))
			dir := ".terraform/modules/" + hex.EncodeToString(sum[:])
			mod, diags := loader.Parser().LoadConfigDir(dir)

			return mod, nil, diags
		},
	))
	if diags.HasErrors() {
		panic(diags)
	}

	// terraform/context NewContext()
	variables := terraform.DefaultVariableValues(cfg.Module.Variables)
	// TODO: backend/local/backend_local backend.ParseVariableValues() (load tfvars and env)
	//       terraform/context variables.Override()

	// terraform/graph_context_walker init()
	variableValues := make(map[string]map[string]cty.Value)
	variableValues[""] = make(map[string]cty.Value)
	for k, iv := range variables {
		variableValues[""][k] = iv.Value
	}

	ctx := terraform.BuiltinEvalContext{
		PathValue: addrs.RootModuleInstance,
		Evaluator: &terraform.Evaluator{
			Config:             cfg,
			VariableValues:     variableValues,
			VariableValuesLock: &sync.Mutex{},
		},
	}

	body, _, diags := cfg.Module.ManagedResources["aws_instance.web"].Config.PartialContent(&hcl.BodySchema{
		Attributes: []hcl.AttributeSchema{
			{
				Name: "instance_type",
			},
		},
	})
	if diags.HasErrors() {
		panic(diags)
	}

	val, hcldiags := ctx.EvaluateExpr(body.Attributes["instance_type"].Expr, cty.DynamicPseudoType, nil)
	if hcldiags.HasErrors() {
		panic(hcldiags)
	}

	var ret string
	err = gocty.FromCtyValue(val, &ret)
	if err != nil {
		panic(err)
	}

	pp.Print(ret)
}
