package downloader

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
)

// FetchFromURL загружает файл по URL и возвращает список строк
func FetchFromURL(url string) ([]string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch URL %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code %d from URL %s", resp.StatusCode, url)
	}

	return readLines(resp.Body)
}

// FetchFromFile читает локальный файл и возвращает список строк
func FetchFromFile(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open file %s: %w", path, err)
	}
	defer file.Close()

	return readLines(file)
}

// FetchAll параллельно загружает несколько источников и объединяет результаты
func FetchAll(sources []string) ([]string, error) {
	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		results []string
		errs    []error
	)

	for _, source := range sources {
		wg.Add(1)
		go func(src string) {
			defer wg.Done()

			var lines []string
			var err error

			// Определяем, URL это или файл
			if strings.HasPrefix(src, "http://") || strings.HasPrefix(src, "https://") {
				lines, err = FetchFromURL(src)
			} else {
				lines, err = FetchFromFile(src)
			}

			mu.Lock()
			defer mu.Unlock()

			if err != nil {
				errs = append(errs, fmt.Errorf("source %s: %w", src, err))
				return
			}

			results = append(results, lines...)
		}(source)
	}

	wg.Wait()

	// Если все источники вернули ошибки
	if len(results) == 0 && len(errs) > 0 {
		return nil, fmt.Errorf("all sources failed: %v", errs)
	}

	return results, nil
}

// readLines читает строки из reader, отбрасывает пустые и комментарии
func readLines(r io.Reader) ([]string, error) {
	var lines []string
	scanner := bufio.NewScanner(r)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Пропускаем пустые строки и комментарии
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		lines = append(lines, line)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading lines: %w", err)
	}

	return lines, nil
}
