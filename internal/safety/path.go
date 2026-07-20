package safety

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
)

var artifactIdentifierPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]*$`)

func ValidateArtifactIdentifier(kind, value string) error {
	if !artifactIdentifierPattern.MatchString(value) {
		return fmt.Errorf("%s %q must match %s", kind, value, artifactIdentifierPattern.String())
	}
	return nil
}

func ValidateExistingPathWithin(rootPath, targetPath string) error {
	_, _, err := resolveWithin(rootPath, targetPath, false)
	return err
}

func MkdirAllWithin(rootPath, targetPath string, perm os.FileMode) (err error) {
	if err := os.MkdirAll(rootPath, perm); err != nil {
		return err
	}
	root, relative, err := openResolvedRoot(rootPath, targetPath, true)
	if err != nil {
		return err
	}
	defer closeWithError(&err, root)
	return root.MkdirAll(relative, perm)
}

func ReadFileWithin(rootPath, targetPath string) (data []byte, err error) {
	root, relative, err := openResolvedRoot(rootPath, targetPath, false)
	if err != nil {
		return nil, err
	}
	defer closeWithError(&err, root)
	return root.ReadFile(relative)
}

func WriteFileWithin(rootPath, targetPath string, data []byte, perm os.FileMode) (err error) {
	root, relative, err := openResolvedRoot(rootPath, targetPath, true)
	if err != nil {
		return err
	}
	defer closeWithError(&err, root)
	return root.WriteFile(relative, data, perm)
}

func OpenFileWithin(rootPath, targetPath string, flag int, perm os.FileMode) (*os.File, error) {
	root, relative, err := openResolvedRoot(rootPath, targetPath, flag&os.O_CREATE != 0)
	if err != nil {
		return nil, err
	}
	file, openErr := root.OpenFile(relative, flag, perm)
	closeErr := root.Close()
	if openErr != nil {
		return nil, errors.Join(openErr, closeErr)
	}
	if closeErr != nil {
		return nil, errors.Join(closeErr, file.Close())
	}
	return file, nil
}

func StatWithin(rootPath, targetPath string) (info os.FileInfo, err error) {
	root, relative, err := openResolvedRoot(rootPath, targetPath, false)
	if err != nil {
		return nil, err
	}
	defer closeWithError(&err, root)
	return root.Stat(relative)
}

func ReadDirWithin(rootPath, targetPath string) (entries []os.DirEntry, err error) {
	root, relative, err := openResolvedRoot(rootPath, targetPath, false)
	if err != nil {
		return nil, err
	}
	defer closeWithError(&err, root)
	dir, err := root.Open(relative)
	if err != nil {
		return nil, err
	}
	defer closeWithError(&err, dir)
	return dir.ReadDir(-1)
}

func closeWithError(result *error, closer io.Closer) {
	*result = errors.Join(*result, closer.Close())
}

func openResolvedRoot(rootPath, targetPath string, allowMissing bool) (*os.Root, string, error) {
	canonicalRoot, relative, err := resolveWithin(rootPath, targetPath, allowMissing)
	if err != nil {
		return nil, "", err
	}
	root, err := os.OpenRoot(canonicalRoot)
	if err != nil {
		return nil, "", err
	}
	return root, relative, nil
}

func resolveWithin(rootPath, targetPath string, allowMissing bool) (string, string, error) {
	rootAbs, err := filepath.Abs(rootPath)
	if err != nil {
		return "", "", err
	}
	canonicalRoot, err := filepath.EvalSymlinks(rootAbs)
	if err != nil {
		return "", "", err
	}
	targetAbs, err := filepath.Abs(targetPath)
	if err != nil {
		return "", "", err
	}
	canonicalTarget, err := resolveTarget(targetAbs, allowMissing)
	if err != nil {
		return "", "", err
	}
	relative, err := filepath.Rel(canonicalRoot, canonicalTarget)
	if err != nil {
		return "", "", err
	}
	if !filepath.IsLocal(relative) {
		return "", "", fmt.Errorf("path %q escapes allowed root %q", targetPath, rootPath)
	}
	return canonicalRoot, relative, nil
}

func resolveTarget(targetPath string, allowMissing bool) (string, error) {
	if !allowMissing {
		return filepath.EvalSymlinks(targetPath)
	}

	current := targetPath
	var missing []string
	for {
		_, err := os.Lstat(current)
		if err == nil {
			resolved, err := filepath.EvalSymlinks(current)
			if err != nil {
				return "", err
			}
			for i := len(missing) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, missing[i])
			}
			return resolved, nil
		}
		if !os.IsNotExist(err) {
			return "", err
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", err
		}
		missing = append(missing, filepath.Base(current))
		current = parent
	}
}
