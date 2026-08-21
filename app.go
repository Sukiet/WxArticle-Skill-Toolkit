package main

import (
	"bytes"
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	envRepoDir = "WX_ARTICLE_REPO_DIR"
	envHost    = "WX_ARTICLE_HOST"
)

type App struct {
	stdout io.Writer
	stderr io.Writer
	client *http.Client
}

func newApp(stdout, stderr io.Writer) *App {
	return &App{
		stdout: stdout,
		stderr: stderr,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (a *App) loadDotEnv() error {
	for _, path := range envCandidatePaths() {
		info, err := os.Stat(path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return fmt.Errorf("stat .env: %w", err)
		}
		if info.IsDir() {
			continue
		}
		if err := loadDotEnvFile(path); err != nil {
			return err
		}
		return nil
	}

	return nil
}

func (a *App) writeJSON(out io.Writer, value any) error {
	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func (a *App) writeError(err error) {
	_ = a.writeJSON(a.stderr, map[string]any{
		"ok":      false,
		"error":   err.Error(),
		"message": "命令执行失败。请先和用户确认是否需要继续处理这个问题，再决定是否重新执行相关命令。",
	})
}

func (a *App) writeSuccess(value map[string]any) error {
	if _, ok := value["ok"]; !ok {
		value["ok"] = true
	}
	return a.writeJSON(a.stdout, value)
}

func (a *App) repoDir() (string, error) {
	repoDir := strings.TrimSpace(os.Getenv(envRepoDir))
	if repoDir == "" {
		return "", fmt.Errorf("missing environment variable %s", envRepoDir)
	}

	absolute, err := filepath.Abs(repoDir)
	if err != nil {
		return "", fmt.Errorf("resolve repo dir: %w", err)
	}

	info, err := os.Stat(absolute)
	if err != nil {
		return "", fmt.Errorf("stat repo dir: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("repo dir is not a directory: %s", absolute)
	}

	return absolute, nil
}

func (a *App) host() (string, error) {
	host := strings.TrimSpace(os.Getenv(envHost))
	if host == "" {
		return "", fmt.Errorf("missing environment variable %s", envHost)
	}
	return strings.TrimRight(host, "/"), nil
}

func makeArticlePaths(repoDir, articleUUID string) articlePaths {
	projectDir := filepath.Join(repoDir, articleUUID)
	return articlePaths{
		ProjectDir:          projectDir,
		ImagesDir:           filepath.Join(projectDir, "images"),
		MetadataPath:        filepath.Join(projectDir, "metadata.json"),
		ExampleMetadataPath: filepath.Join(projectDir, "metadata.example.json"),
		ExampleHTMLPath:     filepath.Join(projectDir, "example.html"),
		ArticleHTMLPath:     filepath.Join(projectDir, "article.html"),
	}
}

func ensureDir(path string) error {
	return os.MkdirAll(path, 0o755)
}

func createEmptyFile(path string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	return file.Close()
}

func writeJSONFile(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

func readJSONFile(path string, value any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, value)
}

func currentTimestamp() float64 {
	return float64(time.Now().UnixNano()) / 1e9
}

func formatTimestamp(timestamp float64) string {
	if timestamp <= 0 {
		return ""
	}

	seconds := int64(timestamp)
	nanos := int64((timestamp - float64(seconds)) * 1e9)
	return time.Unix(seconds, nanos).Local().Format("2006-01-02 15:04:05")
}

func newUUIDv4() (string, error) {
	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	buffer[6] = (buffer[6] & 0x0f) | 0x40
	buffer[8] = (buffer[8] & 0x3f) | 0x80

	return fmt.Sprintf(
		"%08x-%04x-%04x-%04x-%012x",
		buffer[0:4],
		buffer[4:6],
		buffer[6:8],
		buffer[8:10],
		buffer[10:16],
	), nil
}

func fileMD5(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

func isFileEmpty(path string) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	return len(bytes.TrimSpace(data)) == 0, nil
}

func (a *App) doRequest(request *http.Request) (*http.Response, error) {
	response, err := a.client.Do(request)
	if err != nil {
		return nil, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		defer response.Body.Close()
		body, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		if len(body) == 0 {
			return nil, fmt.Errorf("unexpected status %s", response.Status)
		}
		return nil, fmt.Errorf("unexpected status %s: %s", response.Status, strings.TrimSpace(string(body)))
	}
	return response, nil
}

func (a *App) getJSON(url string, value any) error {
	request, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	response, err := a.doRequest(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	return json.NewDecoder(response.Body).Decode(value)
}

func (a *App) getBytes(url string) ([]byte, error) {
	request, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	response, err := a.doRequest(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	return io.ReadAll(response.Body)
}

func (a *App) postJSON(url string, payload any, value any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")

	response, err := a.doRequest(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if value == nil {
		_, err = io.Copy(io.Discard, response.Body)
		return err
	}
	return json.NewDecoder(response.Body).Decode(value)
}

func (a *App) postHTML(url string, content []byte, value any) error {
	request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(content))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "text/html; charset=utf-8")

	response, err := a.doRequest(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if value == nil {
		_, err = io.Copy(io.Discard, response.Body)
		return err
	}
	return json.NewDecoder(response.Body).Decode(value)
}

func (a *App) postMultipart(url string, fields map[string]string, fileField, filePath string, value any) error {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	for key, fieldValue := range fields {
		if strings.TrimSpace(fieldValue) == "" {
			continue
		}
		if err := writer.WriteField(key, fieldValue); err != nil {
			return err
		}
	}

	fileWriter, err := writer.CreateFormFile(fileField, filepath.Base(filePath))
	if err != nil {
		return err
	}

	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	if _, err := io.Copy(fileWriter, file); err != nil {
		return err
	}

	if err := writer.Close(); err != nil {
		return err
	}

	request, err := http.NewRequest(http.MethodPost, url, body)
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", writer.FormDataContentType())

	response, err := a.doRequest(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if value == nil {
		_, err = io.Copy(io.Discard, response.Body)
		return err
	}
	return json.NewDecoder(response.Body).Decode(value)
}

func mustProjectExist(paths articlePaths) error {
	info, err := os.Stat(paths.ProjectDir)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return errors.New("article project path is not a directory")
	}
	return nil
}

func envCandidatePaths() []string {
	seen := map[string]struct{}{}
	var paths []string

	addPath := func(base string) {
		if strings.TrimSpace(base) == "" {
			return
		}
		candidate := filepath.Join(base, ".env")
		candidate = filepath.Clean(candidate)
		if _, exists := seen[candidate]; exists {
			return
		}
		seen[candidate] = struct{}{}
		paths = append(paths, candidate)
	}

	if cwd, err := os.Getwd(); err == nil {
		addPath(cwd)
	}

	if executable, err := os.Executable(); err == nil {
		execDir := filepath.Dir(executable)
		addPath(execDir)
		addPath(filepath.Dir(execDir))
	}

	return paths
}

func loadDotEnvFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read .env file: %w", err)
	}

	lines := strings.Split(string(data), "\n")
	for index, line := range lines {
		line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "export ") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		}

		key, value, found := strings.Cut(line, "=")
		if !found {
			return fmt.Errorf("invalid .env line %d in %s", index+1, path)
		}

		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" {
			return fmt.Errorf("empty key in .env line %d in %s", index+1, path)
		}

		if quoted, err := strconvUnquoteEnvValue(value); err != nil {
			return fmt.Errorf("parse .env line %d in %s: %w", index+1, path, err)
		} else {
			value = quoted
		}

		if _, exists := os.LookupEnv(key); exists {
			continue
		}
		if err := os.Setenv(key, value); err != nil {
			return fmt.Errorf("set env %s from %s: %w", key, path, err)
		}
	}

	return nil
}

func strconvUnquoteEnvValue(value string) (string, error) {
	if len(value) < 2 {
		return value, nil
	}
	if (strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"")) ||
		(strings.HasPrefix(value, "'") && strings.HasSuffix(value, "'")) {
		return value[1 : len(value)-1], nil
	}
	return value, nil
}
