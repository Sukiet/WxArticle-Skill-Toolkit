package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/skip2/go-qrcode"
)

func (a *App) Run(_ context.Context, args []string) int {
	if err := a.run(args); err != nil {
		a.writeError(err)
		return 1
	}
	return 0
}

func (a *App) run(args []string) error {
	specs := a.commandSpecs()

	if len(args) == 0 || wantsHelp(args) && len(args) == 1 {
		a.printRootHelp(specs)
		return nil
	}

	if len(args) == 1 && args[0] == "--version" {
		return a.commandVersion(nil)
	}

	if args[0] == "help" {
		if len(args) == 1 {
			a.printRootHelp(specs)
			return nil
		}

		spec, ok := specs[args[1]]
		if !ok {
			return fmt.Errorf("unknown command: %s", args[1])
		}
		a.printCommandHelp(spec)
		return nil
	}

	spec, ok := specs[args[0]]
	if !ok {
		return fmt.Errorf("unknown command: %s", args[0])
	}
	if wantsHelp(args[1:]) {
		a.printCommandHelp(spec)
		return nil
	}
	if spec.NeedsEnv {
		if err := a.loadDotEnv(); err != nil {
			return err
		}
	}
	return spec.Run(args[1:])
}

func newFlagSet(name string) *flag.FlagSet {
	set := flag.NewFlagSet(name, flag.ContinueOnError)
	set.SetOutput(io.Discard)
	return set
}

func ensureNoExtraArgs(set *flag.FlagSet) error {
	if extras := set.Args(); len(extras) > 0 {
		return fmt.Errorf("unexpected arguments: %s", strings.Join(extras, " "))
	}
	return nil
}

func (a *App) commandVersion(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unexpected arguments: %s", strings.Join(args, " "))
	}
	_, _ = fmt.Fprintln(a.stdout, toolVersion())
	return nil
}

func (a *App) commandInit(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unexpected arguments: %s", strings.Join(args, " "))
	}

	repoDir, err := a.repoDir()
	if err != nil {
		return err
	}

	articleUUID, err := newUUIDv4()
	if err != nil {
		return fmt.Errorf("generate article uuid: %w", err)
	}

	paths := makeArticlePaths(repoDir, articleUUID)
	if _, err := os.Stat(paths.ProjectDir); err == nil {
		return fmt.Errorf("article project already exists: %s", paths.ProjectDir)
	}

	if err := ensureDir(paths.ImagesDir); err != nil {
		return fmt.Errorf("create images dir: %w", err)
	}
	if err := createEmptyFile(paths.ArticleHTMLPath); err != nil {
		return fmt.Errorf("create article.html: %w", err)
	}

	now := currentTimestamp()
	articleMetadata := ArticleMetadata{
		ArticleUUID: articleUUID,
		Title:       "",
		Abstract:    "",
		CreatedAt:   now,
		ModifiedAt:  now,
	}
	if err := writeJSONFile(paths.MetadataPath, articleMetadata); err != nil {
		return fmt.Errorf("write metadata.json: %w", err)
	}

	return a.writeSuccess(map[string]any{
		"article_uuid":      articleUUID,
		"project_dir":       paths.ProjectDir,
		"metadata_path":     paths.MetadataPath,
		"article_html_path": paths.ArticleHTMLPath,
		"suggested_prompt": fmt.Sprintf(
			"已新建文章工程，项目路径在 %s，文章 id 为 %s。如果用户确认要查看或使用模板，接下来可能会用到 templates-list 和 fetch-template。",
			paths.ProjectDir,
			articleUUID,
		),
	})
}

func (a *App) commandWorkDir(args []string) error {
	set := newFlagSet("work-dir")
	articleUUID := set.String("a", "", "")
	set.StringVar(articleUUID, "article", "", "")
	if err := set.Parse(args); err != nil {
		return err
	}
	if err := ensureNoExtraArgs(set); err != nil {
		return err
	}
	if strings.TrimSpace(*articleUUID) == "" {
		return errors.New("missing article uuid: use -a or --article")
	}

	repoDir, err := a.repoDir()
	if err != nil {
		return err
	}

	paths := makeArticlePaths(repoDir, *articleUUID)
	if err := mustProjectExist(paths); err != nil {
		return fmt.Errorf("article project not found: %w", err)
	}

	return a.writeSuccess(map[string]any{
		"article_uuid":          *articleUUID,
		"project_dir":           paths.ProjectDir,
		"metadata_path":         paths.MetadataPath,
		"article_html_path":     paths.ArticleHTMLPath,
		"images_dir":            paths.ImagesDir,
		"example_metadata_path": paths.ExampleMetadataPath,
		"example_html_path":     paths.ExampleHTMLPath,
		"message":               "文章工程路径已定位完成。",
		"next_commands":         []string{"fetch-template", "compress", "upload", "preview"},
	})
}

func (a *App) commandTemplatesList(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unexpected arguments: %s", strings.Join(args, " "))
	}

	host, err := a.host()
	if err != nil {
		return err
	}

	var raw map[string]TemplateSummary
	if err := a.getJSON(host+"/templates_list", &raw); err != nil {
		return fmt.Errorf("fetch templates list: %w", err)
	}

	output := make(map[string]TemplateSummaryOutput, len(raw))
	for uuid, item := range raw {
		output[uuid] = TemplateSummaryOutput{
			Name:               item.Name,
			Intro:              item.Intro,
			CreatedAt:          item.CreatedAt,
			ModifiedAt:         item.ModifiedAt,
			CreatedAtReadable:  formatTimestamp(item.CreatedAt),
			ModifiedAtReadable: formatTimestamp(item.ModifiedAt),
		}
	}

	return a.writeSuccess(map[string]any{
		"message":       "可用模板列表已获取完成，可以用于和用户确认是否需要使用某个模板。",
		"next_commands": []string{"fetch-template"},
		"templates":     output,
	})
}

func (a *App) commandFetchTemplate(args []string) error {
	set := newFlagSet("fetch-template")
	articleUUID := set.String("a", "", "")
	set.StringVar(articleUUID, "article", "", "")
	templateUUID := set.String("t", "", "")
	set.StringVar(templateUUID, "template", "", "")
	if err := set.Parse(args); err != nil {
		return err
	}
	if err := ensureNoExtraArgs(set); err != nil {
		return err
	}
	if strings.TrimSpace(*articleUUID) == "" {
		return errors.New("missing article uuid: use -a or --article")
	}
	if strings.TrimSpace(*templateUUID) == "" {
		return errors.New("missing template uuid: use -t or --template")
	}

	repoDir, err := a.repoDir()
	if err != nil {
		return err
	}
	host, err := a.host()
	if err != nil {
		return err
	}

	paths := makeArticlePaths(repoDir, *articleUUID)
	if err := mustProjectExist(paths); err != nil {
		return fmt.Errorf("article project not found: %w", err)
	}

	var metadata TemplateMetadata
	if err := a.getJSON(host+"/template/"+*templateUUID+"/metadata", &metadata); err != nil {
		return fmt.Errorf("fetch template metadata: %w", err)
	}
	if strings.TrimSpace(metadata.TemplateUUID) == "" {
		metadata.TemplateUUID = *templateUUID
	}

	content, err := a.getBytes(host + "/template/" + *templateUUID + "/content")
	if err != nil {
		return fmt.Errorf("fetch template html: %w", err)
	}

	if err := writeJSONFile(paths.ExampleMetadataPath, metadata); err != nil {
		return fmt.Errorf("write metadata.example.json: %w", err)
	}
	if err := os.WriteFile(paths.ExampleHTMLPath, content, 0o644); err != nil {
		return fmt.Errorf("write example.html: %w", err)
	}

	return a.writeSuccess(map[string]any{
		"article_uuid":          *articleUUID,
		"template_uuid":         metadata.TemplateUUID,
		"example_metadata_path": paths.ExampleMetadataPath,
		"example_html_path":     paths.ExampleHTMLPath,
		"created_at":            metadata.CreatedAt,
		"modified_at":           metadata.ModifiedAt,
		"created_at_readable":   formatTimestamp(metadata.CreatedAt),
		"modified_at_readable":  formatTimestamp(metadata.ModifiedAt),
		"message":               "模板文件已拉取到本地，正式文章文件尚未被修改。",
		"next_commands":         []string{"upload", "preview"},
	})
}

func (a *App) commandUpload(args []string) error {
	set := newFlagSet("upload")
	articleUUID := set.String("a", "", "")
	set.StringVar(articleUUID, "article", "", "")
	if err := set.Parse(args); err != nil {
		return err
	}
	if err := ensureNoExtraArgs(set); err != nil {
		return err
	}
	if strings.TrimSpace(*articleUUID) == "" {
		return errors.New("missing article uuid: use -a or --article")
	}

	repoDir, err := a.repoDir()
	if err != nil {
		return err
	}
	host, err := a.host()
	if err != nil {
		return err
	}

	paths := makeArticlePaths(repoDir, *articleUUID)
	if err := mustProjectExist(paths); err != nil {
		return fmt.Errorf("article project not found: %w", err)
	}

	empty, err := isFileEmpty(paths.ArticleHTMLPath)
	if err != nil {
		return fmt.Errorf("read article.html: %w", err)
	}
	if empty {
		return errors.New("article.html is empty")
	}

	var metadata ArticleMetadata
	if err := readJSONFile(paths.MetadataPath, &metadata); err != nil {
		return fmt.Errorf("read metadata.json: %w", err)
	}
	if strings.TrimSpace(metadata.Title) == "" {
		return errors.New("metadata.json title is required before upload")
	}
	if strings.TrimSpace(metadata.ArticleUUID) == "" {
		metadata.ArticleUUID = *articleUUID
	}

	imageEntries, err := os.ReadDir(paths.ImagesDir)
	if err != nil {
		return fmt.Errorf("read images dir: %w", err)
	}

	sort.Slice(imageEntries, func(i, j int) bool {
		return imageEntries[i].Name() < imageEntries[j].Name()
	})

	uploadedImages := 0
	skippedImages := 0

	for _, entry := range imageEntries {
		if entry.IsDir() {
			continue
		}

		imagePath := filepath.Join(paths.ImagesDir, entry.Name())
		sizeBytes, err := fileSize(imagePath)
		if err != nil {
			return fmt.Errorf("stat image %s: %w", entry.Name(), err)
		}
		limitBytes := imageSizeLimitForPath(imagePath)
		limitLabel := formatSizeLimitLabelForPath(imagePath)
		if sizeBytes > limitBytes {
			return fmt.Errorf("image %s is larger than %s; use compress -a %s -n %s before upload", entry.Name(), limitLabel, *articleUUID, entry.Name())
		}

		md5Value, err := fileMD5(imagePath)
		if err != nil {
			return fmt.Errorf("md5 image %s: %w", entry.Name(), err)
		}

		var syncResponse ImageSyncResponse
		if err := a.postJSON(host+"/sync/"+*articleUUID+"/image", map[string]string{
			"name": entry.Name(),
			"md5":  md5Value,
		}, &syncResponse); err != nil {
			return fmt.Errorf("sync image reference %s: %w", entry.Name(), err)
		}

		switch syncResponse.SyncState {
		case "identical", "synchronized":
			skippedImages++
		case "need_upload":
			uploadedImages++
			if err := a.postMultipart(host+"/upload/"+*articleUUID+"/image", map[string]string{
				"md5":  md5Value,
				"name": entry.Name(),
			}, "file", imagePath, nil); err != nil {
				return fmt.Errorf("upload image %s: %w", entry.Name(), err)
			}
		default:
			return fmt.Errorf("sync image reference %s: unexpected sync_state %q", entry.Name(), syncResponse.SyncState)
		}
	}

	var metadataResponse ArticleMetadata
	if err := a.postJSON(host+"/upload/"+*articleUUID+"/metadata", map[string]any{
		"article_uuid": *articleUUID,
		"title":        metadata.Title,
		"abstract":     metadata.Abstract,
	}, &metadataResponse); err != nil {
		return fmt.Errorf("upload metadata: %w", err)
	}

	content, err := os.ReadFile(paths.ArticleHTMLPath)
	if err != nil {
		return fmt.Errorf("read article.html: %w", err)
	}
	if err := a.postHTML(host+"/upload/"+*articleUUID+"/content", content, nil); err != nil {
		return fmt.Errorf("upload article html: %w", err)
	}

	return a.writeSuccess(map[string]any{
		"article_uuid":         *articleUUID,
		"uploaded_images":      uploadedImages,
		"skipped_images":       skippedImages,
		"uploaded_metadata":    true,
		"uploaded_content":     true,
		"created_at":           metadataResponse.CreatedAt,
		"modified_at":          metadataResponse.ModifiedAt,
		"created_at_readable":  formatTimestamp(metadataResponse.CreatedAt),
		"modified_at_readable": formatTimestamp(metadataResponse.ModifiedAt),
		"message":              "正式文章内容、元数据和图片已上传完成。",
		"next_commands":        []string{"preview"},
	})
}

func (a *App) commandCompress(args []string) error {
	set := newFlagSet("compress")
	articleUUID := set.String("a", "", "")
	set.StringVar(articleUUID, "article", "", "")
	imageName := set.String("n", "", "")
	set.StringVar(imageName, "name", "", "")
	if err := set.Parse(args); err != nil {
		return err
	}
	if err := ensureNoExtraArgs(set); err != nil {
		return err
	}
	if strings.TrimSpace(*articleUUID) == "" {
		return errors.New("missing article uuid: use -a or --article")
	}
	if strings.TrimSpace(*imageName) == "" {
		return errors.New("missing image name: use -n or --name")
	}

	repoDir, err := a.repoDir()
	if err != nil {
		return err
	}

	paths := makeArticlePaths(repoDir, *articleUUID)
	if err := mustProjectExist(paths); err != nil {
		return fmt.Errorf("article project not found: %w", err)
	}

	cleanName := filepath.Base(*imageName)
	if cleanName != *imageName {
		return errors.New("image name must be a direct file name under images")
	}

	imagePath := filepath.Join(paths.ImagesDir, cleanName)
	if _, err := os.Stat(imagePath); err != nil {
		return fmt.Errorf("image not found: %w", err)
	}

	limitBytes := imageSizeLimitForPath(imagePath)
	limitLabel := formatSizeLimitLabelForPath(imagePath)
	beforeSize, afterSize, err := compressImageToLimit(imagePath, maxUploadImageSize, maxCompressedImageWidth)
	if err != nil {
		return fmt.Errorf("compress image %s: %w", cleanName, err)
	}

	nextCommands := []string{"upload"}
	message := fmt.Sprintf("这张图片当前既没有超过 %s，宽度也没有超过 1080px，因此这次没有执行压缩。", limitLabel)
	if afterSize < beforeSize {
		message = fmt.Sprintf("图片已压缩完成，并已覆盖原文件；本次压缩会把宽度控制在 1080px 内，并尽量把体积压到当前格式上限以内（%s），同时尽量保留画质。", limitLabel)
	}

	return a.writeSuccess(map[string]any{
		"article_uuid":  *articleUUID,
		"image_name":    cleanName,
		"image_path":    imagePath,
		"size_before":   beforeSize,
		"size_after":    afterSize,
		"limit_bytes":   limitBytes,
		"limit_label":   limitLabel,
		"message":       message,
		"next_commands": nextCommands,
	})
}

func (a *App) commandPreview(args []string) error {
	set := newFlagSet("preview")
	articleUUID := set.String("a", "", "")
	set.StringVar(articleUUID, "article", "", "")
	if err := set.Parse(args); err != nil {
		return err
	}
	if err := ensureNoExtraArgs(set); err != nil {
		return err
	}
	if strings.TrimSpace(*articleUUID) == "" {
		return errors.New("missing article uuid: use -a or --article")
	}

	repoDir, err := a.repoDir()
	if err != nil {
		return err
	}
	host, err := a.host()
	if err != nil {
		return err
	}

	paths := makeArticlePaths(repoDir, *articleUUID)
	if err := mustProjectExist(paths); err != nil {
		return fmt.Errorf("article project not found: %w", err)
	}

	cacheDir := filepath.Join(repoDir, ".cache")
	if err := ensureDir(cacheDir); err != nil {
		return fmt.Errorf("create cache dir: %w", err)
	}

	previewURL := host + "/preview/" + *articleUUID
	qrCodePath := filepath.Join(cacheDir, *articleUUID+".png")
	if err := qrcode.WriteFile(previewURL, qrcode.Medium, 256, qrCodePath); err != nil {
		return fmt.Errorf("generate qr code: %w", err)
	}

	return a.writeSuccess(map[string]any{
		"article_uuid":     *articleUUID,
		"preview_url":      previewURL,
		"qr_code_path":     qrCodePath,
		"message":          "预览链接和二维码已生成完成。",
		"suggested_prompt": "如果用户确认要查看预览，可以展示二维码图片或使用 preview_url；如果可以，直接在你的对话框中展示给用户看。",
		"next_commands":    []string{"upload", "preview"},
	})
}
