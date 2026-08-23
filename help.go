package main

import (
	"fmt"
	"sort"
	"strings"
)

type CommandSpec struct {
	Name        string
	Summary     string
	Usage       string
	Examples    []string
	NeedsEnv    bool
	Run         func(args []string) error
	ArgsHint    string
	Description string
}

func mustCommandSpec(spec CommandSpec) CommandSpec {
	if strings.TrimSpace(spec.Name) == "" {
		panic("command spec missing Name")
	}
	if strings.TrimSpace(spec.Summary) == "" {
		panic("command spec missing Summary: " + spec.Name)
	}
	if strings.TrimSpace(spec.Usage) == "" {
		panic("command spec missing Usage: " + spec.Name)
	}
	if strings.TrimSpace(spec.Description) == "" {
		panic("command spec missing Description: " + spec.Name)
	}
	if spec.Run == nil {
		panic("command spec missing Run: " + spec.Name)
	}
	return spec
}

func (a *App) commandSpecs() map[string]CommandSpec {
	specs := map[string]CommandSpec{
		"version": mustCommandSpec(CommandSpec{
			Name:        "version",
			Summary:     "输出当前工具版本",
			Usage:       "skill-tool version",
			Examples:    []string{"skill-tool version", "skill-tool --version"},
			NeedsEnv:    false,
			Run:         a.commandVersion,
			Description: "输出当前 skill-tool 二进制版本。",
		}),
		"init": mustCommandSpec(CommandSpec{
			Name:        "init",
			Summary:     "新建文章工程",
			Usage:       "skill-tool init",
			Examples:    []string{"skill-tool init"},
			NeedsEnv:    true,
			Run:         a.commandInit,
			Description: "在仓库目录下创建新的文章工程，只初始化正式文章所需文件。",
		}),
		"work-dir": mustCommandSpec(CommandSpec{
			Name:        "work-dir",
			Summary:     "返回文章工程路径",
			Usage:       "skill-tool work-dir -a {article_uuid}",
			Examples:    []string{"skill-tool work-dir -a 54b988de-d6fd-4163-a7c5-d13d6aace4fc"},
			NeedsEnv:    true,
			Run:         a.commandWorkDir,
			ArgsHint:    "-a, --article",
			Description: "返回某篇文章的本地工程路径以及关键文件路径。",
		}),
		"templates-list": mustCommandSpec(CommandSpec{
			Name:        "templates-list",
			Summary:     "查看可用模板列表",
			Usage:       "skill-tool templates-list",
			Examples:    []string{"skill-tool templates-list"},
			NeedsEnv:    true,
			Run:         a.commandTemplatesList,
			Description: "从后端获取可用模板列表，供用户确认是否使用模板。",
		}),
		"fetch-template": mustCommandSpec(CommandSpec{
			Name:        "fetch-template",
			Summary:     "拉取模板到本地工程",
			Usage:       "skill-tool fetch-template -a {article_uuid} -t {template_uuid}",
			Examples:    []string{"skill-tool fetch-template -a 54b988de-d6fd-4163-a7c5-d13d6aace4fc -t 681f9efe-437f-4fda-91f6-088e4c609868"},
			NeedsEnv:    true,
			Run:         a.commandFetchTemplate,
			ArgsHint:    "-a, --article  -t, --template",
			Description: "拉取模板元数据和模板示例 HTML，不修改正式文章文件。",
		}),
		"compress": mustCommandSpec(CommandSpec{
			Name:        "compress",
			Summary:     "压缩超大图片",
			Usage:       "skill-tool compress -a {article_uuid} -n {image_name}",
			Examples:    []string{"skill-tool compress -a 54b988de-d6fd-4163-a7c5-d13d6aace4fc -n cover.png"},
			NeedsEnv:    true,
			Run:         a.commandCompress,
			ArgsHint:    "-a, --article  -n, --name",
			Description: "仅在图片超过 10MB 时执行压缩，并尽量保留画质。",
		}),
		"upload": mustCommandSpec(CommandSpec{
			Name:        "upload",
			Summary:     "上传文章内容与图片",
			Usage:       "skill-tool upload -a {article_uuid}",
			Examples:    []string{"skill-tool upload -a 54b988de-d6fd-4163-a7c5-d13d6aace4fc"},
			NeedsEnv:    true,
			Run:         a.commandUpload,
			ArgsHint:    "-a, --article",
			Description: "上传正式文章内容、元数据和图片；如果有图片超过 10MB 会先中止。",
		}),
		"preview": mustCommandSpec(CommandSpec{
			Name:        "preview",
			Summary:     "生成预览二维码",
			Usage:       "skill-tool preview -a {article_uuid}",
			Examples:    []string{"skill-tool preview -a 54b988de-d6fd-4163-a7c5-d13d6aace4fc"},
			NeedsEnv:    true,
			Run:         a.commandPreview,
			ArgsHint:    "-a, --article",
			Description: "生成预览链接和二维码，便于在对话中展示给用户。",
		}),
	}

	validateCommandSpecs(specs)
	return specs
}

func validateCommandSpecs(specs map[string]CommandSpec) {
	if len(specs) == 0 {
		panic("no commands registered")
	}
	for name, spec := range specs {
		if name != spec.Name {
			panic("command registry key mismatch: " + name)
		}
	}
}

func (a *App) writeHelpLine(format string, args ...any) {
	_, _ = fmt.Fprintf(a.stdout, format, args...)
}

func (a *App) printRootHelp(specs map[string]CommandSpec) {
	names := sortedCommandNames(specs)

	a.writeHelpLine("skill-tool %s\n\n", toolVersion())
	a.writeHelpLine("用途：管理微信文章工程的本地工具。\n\n")
	a.writeHelpLine("用法：\n")
	a.writeHelpLine("  skill-tool --help\n")
	a.writeHelpLine("  skill-tool help <command>\n")
	a.writeHelpLine("  skill-tool <command> [args]\n\n")
	a.writeHelpLine("环境变量：\n")
	a.writeHelpLine("  %s   本地文章工程根目录\n", envRepoDir)
	a.writeHelpLine("  %s      后端服务地址\n", envHost)
	a.writeHelpLine("  %s             全局统一明文保存的 Bearer token\n\n", envAccessToken)
	a.writeHelpLine("命令：\n")
	for _, name := range names {
		spec := specs[name]
		a.writeHelpLine("  %-15s %s\n", spec.Name, spec.Summary)
	}
	a.writeHelpLine("\n示例：\n")
	a.writeHelpLine("  skill-tool init\n")
	a.writeHelpLine("  skill-tool help fetch-template\n")
	a.writeHelpLine("  skill-tool fetch-template -a {article_uuid} -t {template_uuid}\n")
}

func (a *App) printCommandHelp(spec CommandSpec) {
	a.writeHelpLine("%s\n\n", spec.Name)
	a.writeHelpLine("%s\n\n", spec.Description)
	a.writeHelpLine("用法：\n")
	a.writeHelpLine("  %s\n\n", spec.Usage)
	if spec.ArgsHint != "" {
		a.writeHelpLine("关键参数：\n")
		a.writeHelpLine("  %s\n\n", spec.ArgsHint)
	}
	if len(spec.Examples) > 0 {
		a.writeHelpLine("示例：\n")
		for _, example := range spec.Examples {
			a.writeHelpLine("  %s\n", example)
		}
		a.writeHelpLine("\n")
	}
	a.writeHelpLine("帮助：\n")
	a.writeHelpLine("  skill-tool help %s\n", spec.Name)
	a.writeHelpLine("  skill-tool %s --help\n", spec.Name)
}

func sortedCommandNames(specs map[string]CommandSpec) []string {
	names := make([]string, 0, len(specs))
	for name := range specs {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func wantsHelp(args []string) bool {
	for _, arg := range args {
		if arg == "--help" || arg == "-h" {
			return true
		}
	}
	return false
}
