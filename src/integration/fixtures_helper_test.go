package integration_test

import (
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const (
	javaTestAppsReleaseTag = "v1.0.0-SNAPSHOT"
	// stokpop fork — upstream cloudfoundry/java-test-applications pending PR merge
	javaTestAppsBaseURL = "https://github.com/stokpop/java-test-applications/releases/download/" + javaTestAppsReleaseTag
	sb3JarName             = "java-main-application-boot3-1.0.0-SNAPSHOT.jar"
	sb4JarName             = "java-main-application-1.0.0-SNAPSHOT.jar"
)

// downloadJavaTestAppsJars downloads the pinned SB3 and SB4 fat jars and
// extracts each into a fixture directory, simulating what real CF staging does:
// CF treats the pushed artifact as a zip and extracts it before running the
// buildpack, so BOOT-INF/ and META-INF/ appear as flat files on disk.
//
// Pre-exploding is the correct pattern for Docker-mode tests: switchblade's
// Docker mode archives the fixture directory as-is (TGZArchiver), while CF mode
// delegates to `cf push -p` which handles zip extraction natively.
// See https://github.com/cloudfoundry/switchblade/issues/134 for details.
//
// Returns (sb3FixtureDir, sb4FixtureDir, cleanup func, error).
func downloadJavaTestAppsJars() (string, string, func(), error) {
	dir, err := os.MkdirTemp("", "java-test-apps-*")
	if err != nil {
		return "", "", nil, fmt.Errorf("create temp dir: %w", err)
	}

	cleanup := func() { os.RemoveAll(dir) }

	sb3Dir := filepath.Join(dir, "sb3")
	sb4Dir := filepath.Join(dir, "sb4")

	for _, entry := range []struct {
		jarName string
		destDir string
	}{
		{sb3JarName, sb3Dir},
		{sb4JarName, sb4Dir},
	} {
		jarPath := filepath.Join(dir, entry.jarName)
		if err := downloadFile(javaTestAppsBaseURL+"/"+entry.jarName, jarPath); err != nil {
			cleanup()
			return "", "", nil, fmt.Errorf("download %s: %w", entry.jarName, err)
		}
		if err := extractZip(jarPath, entry.destDir); err != nil {
			cleanup()
			return "", "", nil, fmt.Errorf("extract %s: %w", entry.jarName, err)
		}
		// Remove the fat jar after extraction; the exploded layout is what CF staging produces.
		_ = os.Remove(jarPath)
	}

	return sb3Dir, sb4Dir, cleanup, nil
}

// extractZip extracts a zip/jar file into destDir, replicating the flat-file
// layout that CF staging creates when it unpacks the pushed artifact.
func extractZip(src, destDir string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return fmt.Errorf("open zip: %w", err)
	}
	defer r.Close()

	for _, f := range r.File {
		target := filepath.Join(destDir, f.Name)

		// Guard against zip-slip
		if !strings.HasPrefix(filepath.Clean(target)+string(os.PathSeparator),
			filepath.Clean(destDir)+string(os.PathSeparator)) {
			return fmt.Errorf("zip entry %q escapes destination", f.Name)
		}

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}

		out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			out.Close()
			return err
		}

		_, copyErr := io.Copy(out, rc)
		rc.Close()
		out.Close()
		if copyErr != nil {
			return copyErr
		}
	}
	return nil
}

func downloadFile(url, dest string) error {
	resp, err := http.Get(url) //nolint:noctx
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d for %s", resp.StatusCode, url)
	}

	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(f, resp.Body)
	return err
}
