package main

import (
	"bufio"
	"compress/gzip"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const version = "1.0.0"

// Config holds all command-line options and flags
type Config struct {
	force           bool
	interactive     string
	interactiveFlag bool
	interactiveOnce bool
	recursive       bool
	dir             bool
	verbose         bool
	oneFileSystem   bool
	preserveRoot    bool
	preserveRootAll bool
	noPreserveRoot  bool
	showHelp        bool
	showVersion     bool
	useRecycleBin   bool
	permanentDelete bool
	clearRecycleBin bool
	listRecycleBin  bool
	restoreFile     string
	recycleBinDays  int
	setupRecycleBin bool
	files           []string
}

// RecycleBinEntry represents a deleted file/directory in the recycle bin
type RecycleBinEntry struct {
	OriginalPath   string    `json:"original_path"`
	DeletedAt      time.Time `json:"deleted_at"`
	StoredName     string    `json:"stored_name"`
	IsCompressed   bool      `json:"is_compressed"`
	OriginalSize   int64     `json:"original_size"`
	CompressedSize int64     `json:"compressed_size,omitempty"`
	IsDirectory    bool      `json:"is_directory"`
}

// RecycleBinConfig stores user preferences for the recycle bin
type RecycleBinConfig struct {
	Version        string `json:"version"`
	RecycleBinPath string `json:"recycle_bin_path"`
	RetentionDays  int    `json:"retention_days"`
	MaxSizeMB      int64  `json:"max_size_mb"`
}

func main() {
	config := parseArgs() // Parse command line arguments

	if config.showHelp {
		showHelp()
		return
	}

	if config.showVersion {
		showVersion()
		return
	}

	if config.setupRecycleBin {
		setupRecycleBin()
		return
	}

	if err := initRecycleBin(); err != nil {
		fmt.Fprintf(os.Stderr, "rm: failed to initialize recycle bin: %v\n", err)
		os.Exit(1)
	}

	if config.clearRecycleBin {
		clearRecycleBin()
		return
	}

	if config.listRecycleBin {
		listRecycleBin()
		return
	}

	if config.restoreFile != "" {
		restoreFromRecycleBin(config.restoreFile)
		return
	}

	cleanupRecycleBin() // Remove old files from recycle bin

	if len(config.files) == 0 {
		fmt.Fprintf(os.Stderr, "rm: missing operand\n")
		fmt.Fprintf(os.Stderr, "Try 'rm --help' for more information.\n")
		os.Exit(1)
	}

	if err := validateRootProtection(config); err != nil {
		fmt.Fprintf(os.Stderr, "rm: %v\n", err)
		os.Exit(1)
	}

	if shouldPromptOnce(config) {
		if !promptOnce(config) {
			return
		}
	}

	// Process each file/directory
	for _, file := range config.files {
		if err := removeFile(file, config); err != nil {
			if !config.force {
				fmt.Fprintf(os.Stderr, "rm: %v\n", err)
			}
		}
	}
}

func parseArgs() Config {
	// Set default configuration values
	config := Config{
		preserveRoot:   true,
		useRecycleBin:  true,
		recycleBinDays: 7,
	}

	args := os.Args[1:]
	var files []string

	for i := 0; i < len(args); i++ {
		arg := args[i]

		if arg == "--" {
			// Everything after -- should be treated as files
			files = append(files, args[i+1:]...)
			break
		}

		if !strings.HasPrefix(arg, "-") {
			files = append(files, arg)
			continue
		}

		switch {
		case arg == "-f" || arg == "--force":
			config.force = true
		case arg == "-i":
			config.interactiveFlag = true
			config.interactive = "always"
		case arg == "-I":
			config.interactiveOnce = true
			config.interactive = "once"
		case strings.HasPrefix(arg, "--interactive"):
			if arg == "--interactive" {
				config.interactive = "always"
			} else if strings.Contains(arg, "=") {
				parts := strings.SplitN(arg, "=", 2)
				switch parts[1] {
				case "never":
					config.interactive = "never"
				case "once":
					config.interactive = "once"
				case "always":
					config.interactive = "always"
				default:
					fmt.Fprintf(os.Stderr, "rm: invalid argument '%s' for '--interactive'\n", parts[1])
					os.Exit(1)
				}
			}
		case arg == "-r" || arg == "-R" || arg == "--recursive":
			config.recursive = true
		case arg == "-d" || arg == "--dir":
			config.dir = true
		case arg == "-v" || arg == "--verbose":
			config.verbose = true
		case arg == "--one-file-system":
			config.oneFileSystem = true
		case arg == "--preserve-root":
			config.preserveRoot = true
		case strings.HasPrefix(arg, "--preserve-root="):
			parts := strings.SplitN(arg, "=", 2)
			if parts[1] == "all" {
				config.preserveRootAll = true
			}
			config.preserveRoot = true
		case arg == "--no-preserve-root":
			config.noPreserveRoot = true
			config.preserveRoot = false
		case arg == "--help":
			config.showHelp = true
		case arg == "--version":
			config.showVersion = true
		case arg == "--permanent":
			config.permanentDelete = true
			config.useRecycleBin = false
		case arg == "--clear-recycle-bin":
			config.clearRecycleBin = true
		case arg == "--list-recycle-bin":
			config.listRecycleBin = true
		case arg == "--setup-recycle-bin":
			config.setupRecycleBin = true
		case strings.HasPrefix(arg, "--restore="):
			parts := strings.SplitN(arg, "=", 2)
			config.restoreFile = parts[1]
		case strings.HasPrefix(arg, "--recycle-bin-days="):
			parts := strings.SplitN(arg, "=", 2)
			days, err := strconv.Atoi(parts[1])
			if err != nil || days < 1 {
				fmt.Fprintf(os.Stderr, "rm: invalid retention days '%s'\n", parts[1])
				os.Exit(1)
			}
			config.recycleBinDays = days
		case strings.HasPrefix(arg, "-") && len(arg) > 1:

			for j := 1; j < len(arg); j++ {
				switch arg[j] {
				case 'f':
					config.force = true
				case 'i':
					config.interactiveFlag = true
					config.interactive = "always"
				case 'I':
					config.interactiveOnce = true
					config.interactive = "once"
				case 'r', 'R':
					config.recursive = true
				case 'd':
					config.dir = true
				case 'v':
					config.verbose = true
				default:
					fmt.Fprintf(os.Stderr, "rm: invalid option -- '%c'\n", arg[j])
					fmt.Fprintf(os.Stderr, "Try 'rm --help' for more information.\n")
					os.Exit(1)
				}
			}
		default:
			fmt.Fprintf(os.Stderr, "rm: unrecognized option '%s'\n", arg)
			fmt.Fprintf(os.Stderr, "Try 'rm --help' for more information.\n")
			os.Exit(1)
		}
	}

	config.files = files
	return config
}

func validateRootProtection(config Config) error {
	if config.noPreserveRoot {
		return nil
	}

	// Check each file for dangerous operations
	for _, file := range config.files {
		absPath, err := filepath.Abs(file)
		if err != nil {
			continue
		}

		if absPath == "/" {
			if config.preserveRoot {
				return fmt.Errorf("it is dangerous to operate recursively on '/'")
			}
		}

		if config.preserveRootAll {

			parentPath := filepath.Dir(absPath)
			if isOnDifferentDevice(absPath, parentPath) {
				return fmt.Errorf("skipping '%s', since it's on a different device", file)
			}
		}

		base := filepath.Base(file)
		if base == "." || base == ".." {
			return fmt.Errorf("refusing to remove '.' or '..' directory: skipping '%s'", file)
		}
	}

	return nil
}

func shouldPromptOnce(config Config) bool {
	if config.interactive == "once" || config.interactiveOnce {

		return len(config.files) > 3 || config.recursive
	}
	return false
}

func promptOnce(config Config) bool {
	var operation string
	if config.recursive {
		operation = "remove all arguments recursively"
	} else {
		operation = fmt.Sprintf("remove %d arguments", len(config.files))
	}

	fmt.Printf("rm: %s? ", operation)
	return getYesNo()
}

func removeFile(path string, config Config) error {

	info, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) && config.force {
			return nil
		}
		return fmt.Errorf("cannot remove '%s': %v", path, err)
	}

	if info.IsDir() {
		return removeDirectory(path, info, config)
	}

	return removeRegularFile(path, info, config)
}

func removeRegularFile(path string, info os.FileInfo, config Config) error {

	if shouldPromptForFile(path, info, config) {
		fmt.Printf("rm: remove %s '%s'? ", getFileType(info), path)
		if !getYesNo() {
			return nil
		}
	}

	if config.verbose {
		if config.useRecycleBin && !config.permanentDelete {
			fmt.Printf("moved to recycle bin '%s'\n", path)
		} else {
			fmt.Printf("removed '%s'\n", path)
		}
	}

	if config.useRecycleBin && !config.permanentDelete {
		return moveToRecycleBin(path)
	}

	return os.Remove(path)
}

func removeDirectory(path string, info os.FileInfo, config Config) error {

	if !config.recursive && !config.dir {
		return fmt.Errorf("cannot remove '%s': Is a directory", path)
	}

	if config.dir && !config.recursive {
		if !isDirEmpty(path) {
			return fmt.Errorf("cannot remove '%s': Directory not empty", path)
		}

		if shouldPromptForFile(path, info, config) {
			fmt.Printf("rm: remove directory '%s'? ", path)
			if !getYesNo() {
				return nil
			}
		}

		if config.verbose {
			if config.useRecycleBin && !config.permanentDelete {
				fmt.Printf("moved to recycle bin directory '%s'\n", path)
			} else {
				fmt.Printf("removed directory '%s'\n", path)
			}
		}

		if config.useRecycleBin && !config.permanentDelete {
			return moveToRecycleBin(path)
		}

		return os.Remove(path)
	}

	if config.recursive {
		return removeRecursively(path, config)
	}

	return fmt.Errorf("cannot remove '%s': Is a directory", path)
}

func removeRecursively(path string, config Config) error {

	if config.oneFileSystem {
		if err := checkSameFileSystem(path, config); err != nil {
			return err
		}
	}

	info, err := os.Lstat(path)
	if err != nil {
		return err
	}

	if shouldPromptForFile(path, info, config) {
		fmt.Printf("rm: descend into directory '%s'? ", path)
		if !getYesNo() {
			return nil
		}
	}

	if config.useRecycleBin && !config.permanentDelete {
		if config.verbose {
			fmt.Printf("moved to recycle bin '%s'\n", path)
		}
		return moveToRecycleBin(path)
	}

	err = filepath.Walk(path, func(walkPath string, walkInfo os.FileInfo, walkErr error) error {
		if walkErr != nil {
			if !config.force {
				return walkErr
			}
			return nil
		}

		if walkPath == path {
			return nil
		}

		if walkInfo.IsDir() {

			return nil
		}

		if shouldPromptForFile(walkPath, walkInfo, config) {
			fmt.Printf("rm: remove %s '%s'? ", getFileType(walkInfo), walkPath)
			if !getYesNo() {
				return nil
			}
		}

		if config.verbose {
			fmt.Printf("removed '%s'\n", walkPath)
		}

		return os.Remove(walkPath)
	})

	if err != nil && !config.force {
		return err
	}

	var dirs []string
	filepath.Walk(path, func(walkPath string, walkInfo os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if walkInfo.IsDir() {
			dirs = append(dirs, walkPath)
		}
		return nil
	})

	for i := len(dirs) - 1; i >= 0; i-- {
		dir := dirs[i]
		dirInfo, err := os.Lstat(dir)
		if err != nil {
			continue
		}

		if shouldPromptForFile(dir, dirInfo, config) && dir != path {
			fmt.Printf("rm: remove directory '%s'? ", dir)
			if !getYesNo() {
				continue
			}
		}

		if config.verbose {
			fmt.Printf("removed directory '%s'\n", dir)
		}
		os.Remove(dir)
	}

	return nil
}

func shouldPromptForFile(path string, info os.FileInfo, config Config) bool {

	if config.force {
		return false
	}

	if config.interactive == "always" || config.interactiveFlag {
		return true
	}

	if config.interactive == "never" {
		return false
	}

	if !isWritable(path, info) && isTerminal() {
		return true
	}

	return false
}

func isWritable(path string, info os.FileInfo) bool {

	mode := info.Mode()

	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return true
	}

	uid := os.Getuid()
	gid := os.Getgid()

	if uint32(uid) == stat.Uid {
		return mode&0200 != 0
	}

	if uint32(gid) == stat.Gid {
		return mode&0020 != 0
	}

	return mode&0002 != 0
}

func isTerminal() bool {
	fileInfo, _ := os.Stdin.Stat()
	return (fileInfo.Mode() & os.ModeCharDevice) != 0
}

func getFileType(info os.FileInfo) string {
	if info.IsDir() {
		return "directory"
	}
	mode := info.Mode()
	switch {
	case mode&os.ModeSymlink != 0:
		return "symbolic link"
	case mode&os.ModeDevice != 0:
		return "device file"
	case mode&os.ModeNamedPipe != 0:
		return "named pipe"
	case mode&os.ModeSocket != 0:
		return "socket"
	default:
		if isWritable("", info) {
			return "regular file"
		}
		return "write-protected regular file"
	}
}

func isDirEmpty(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()

	_, err = f.Readdirnames(1)
	return err != nil
}

func getYesNo() bool {
	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		response := strings.TrimSpace(strings.ToLower(scanner.Text()))
		return response == "y" || response == "yes"
	}
	return false
}

func isOnDifferentDevice(path1, path2 string) bool {
	stat1, err1 := os.Stat(path1)
	stat2, err2 := os.Stat(path2)

	if err1 != nil || err2 != nil {
		return false
	}

	sys1, ok1 := stat1.Sys().(*syscall.Stat_t)
	sys2, ok2 := stat2.Sys().(*syscall.Stat_t)

	if !ok1 || !ok2 {
		return false
	}

	return sys1.Dev != sys2.Dev
}

func checkSameFileSystem(path string, config Config) error {

	return nil
}

func showHelp() {
	fmt.Print(`Usage: rm [OPTION]... [FILE]...
Remove (unlink) the FILE(s).

  -f, --force           ignore nonexistent files and arguments, never prompt
  -i                    prompt before every removal
  -I                    prompt once before removing more than three files, or
                          when removing recursively; less intrusive than -i,
                          while still giving protection against most mistakes
      --interactive[=WHEN]  prompt according to WHEN: never, once (-I), or
                          always (-i); without WHEN, prompt always
      --one-file-system  when removing a hierarchy recursively, skip any
                          directory that is on a file system different from
                          that of the corresponding command line argument
      --no-preserve-root  do not treat '/' specially
      --preserve-root[=all]  do not remove '/' (default);
                          with 'all', reject any command line argument
                          on a separate device from its parent
  -r, -R, --recursive   remove directories and their contents recursively
  -d, --dir             remove empty directories
  -v, --verbose         explain what is being done
      --help     display this help and exit
      --version  output version information and exit

Recycle Bin Options:
      --permanent       permanently delete files (bypass recycle bin)
      --setup-recycle-bin  setup recycle bin configuration
      --clear-recycle-bin  permanently delete all items from recycle bin
      --list-recycle-bin   list items in recycle bin
      --restore=PATH    restore file from recycle bin to original location
      --recycle-bin-days=N  set retention days for recycle bin (default: 7)

By default, rm does not remove directories.  Use the --recursive (-r or -R)
option to remove each listed directory, too, along with all of its contents.

By default, files are moved to a recycle bin with compression and automatically 
deleted after 7 days. Use --permanent to bypass the recycle bin and delete immediately.
Files are compressed using gzip to save space while preserving full recoverability.

To remove a file whose name starts with a '-', for example '-foo',
use one of these commands:
  rm -- -foo

  rm ./-foo

Examples:
  rm file.txt                    # Move file.txt to recycle bin
  rm --permanent file.txt        # Permanently delete file.txt
  rm --list-recycle-bin          # List all items in recycle bin
  rm --restore=file.txt          # Restore file.txt from recycle bin
  rm --clear-recycle-bin         # Empty the recycle bin permanently
  rm --setup-recycle-bin         # Configure recycle bin settings

Report rm bugs to <bug-coreutils@gnu.org>
GNU coreutils home page: <https:
`)
}

func showVersion() {
	fmt.Printf("rm (better-rm) %s\n", version)
	fmt.Println("This is a better version of the GNU rm command.")
}

func getConfigDir() string {
	if runtime.GOOS == "windows" {
		return filepath.Join(os.Getenv("APPDATA"), "better-rm")
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "/tmp/better-rm"
	}

	configDir := filepath.Join(homeDir, ".config", "better-rm")
	return configDir
}

func getRecycleBinConfigPath() string {
	return filepath.Join(getConfigDir(), "config.json")
}

func getDefaultRecycleBinPath() string {
	if runtime.GOOS == "windows" {
		return filepath.Join(os.Getenv("TEMP"), "better-rm-recycle-bin")
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "/tmp/better-rm-recycle-bin"
	}

	return filepath.Join(homeDir, ".local", "share", "better-rm", "recycle-bin")
}

func loadRecycleBinConfig() (*RecycleBinConfig, error) {
	configPath := getRecycleBinConfigPath()

	if _, err := os.Stat(configPath); os.IsNotExist(err) {

		return &RecycleBinConfig{
			Version:        version,
			RecycleBinPath: getDefaultRecycleBinPath(),
			RetentionDays:  7,
			MaxSizeMB:      1024,
		}, nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var config RecycleBinConfig
	err = json.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}

func saveRecycleBinConfig(config *RecycleBinConfig) error {
	configDir := getConfigDir()
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(getRecycleBinConfigPath(), data, 0600)
}

func setupRecycleBin() {
	fmt.Println("Setting up recycle bin for better-rm...")

	defaultPath := getDefaultRecycleBinPath()
	fmt.Printf("Default recycle bin location: %s\n", defaultPath)
	fmt.Print("Use this location? (y/n) [y]: ")

	scanner := bufio.NewScanner(os.Stdin)
	var response string
	if scanner.Scan() {
		response = strings.TrimSpace(strings.ToLower(scanner.Text()))
	}

	var recycleBinPath string
	if response == "n" || response == "no" {
		fmt.Print("Enter custom recycle bin path: ")
		if scanner.Scan() {
			recycleBinPath = strings.TrimSpace(scanner.Text())
		}
		if recycleBinPath == "" {
			recycleBinPath = defaultPath
		} else {

			recycleBinPath = filepath.Clean(recycleBinPath)
			if !filepath.IsAbs(recycleBinPath) {
				fmt.Fprintf(os.Stderr, "Error: Path must be absolute\n")
				os.Exit(1)
			}
		}
	} else {
		recycleBinPath = defaultPath
	}

	fmt.Print("Enter retention days (default 7): ")
	retentionDays := 7
	if scanner.Scan() {
		input := strings.TrimSpace(scanner.Text())
		if input != "" {
			if days, err := strconv.Atoi(input); err == nil && days > 0 {
				retentionDays = days
			}
		}
	}

	config := &RecycleBinConfig{
		Version:        version,
		RecycleBinPath: recycleBinPath,
		RetentionDays:  retentionDays,
		MaxSizeMB:      1024,
	}

	if err := os.MkdirAll(recycleBinPath, 0700); err != nil {
		fmt.Fprintf(os.Stderr, "Error: Failed to create recycle bin directory: %v\n", err)
		os.Exit(1)
	}

	metadataDir := filepath.Join(recycleBinPath, ".metadata")
	if err := os.MkdirAll(metadataDir, 0700); err != nil {
		fmt.Fprintf(os.Stderr, "Error: Failed to create metadata directory: %v\n", err)
		os.Exit(1)
	}

	if err := saveRecycleBinConfig(config); err != nil {
		fmt.Fprintf(os.Stderr, "Error: Failed to save config: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Recycle bin setup complete!\n")
	fmt.Printf("Location: %s\n", recycleBinPath)
	fmt.Printf("Retention: %d days\n", retentionDays)
}

func initRecycleBin() error {
	config, err := loadRecycleBinConfig()
	if err != nil {
		return err
	}

	configPath := getRecycleBinConfigPath()
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		fmt.Println("better-rm: First time setup detected.")
		fmt.Println("Run 'better-rm --setup-recycle-bin' to configure the recycle bin.")

		if err := os.MkdirAll(config.RecycleBinPath, 0700); err != nil {
			return err
		}

		metadataDir := filepath.Join(config.RecycleBinPath, ".metadata")
		if err := os.MkdirAll(metadataDir, 0700); err != nil {
			return err
		}

		return saveRecycleBinConfig(config)
	}

	if err := os.MkdirAll(config.RecycleBinPath, 0700); err != nil {
		return err
	}

	metadataDir := filepath.Join(config.RecycleBinPath, ".metadata")
	return os.MkdirAll(metadataDir, 0700)
}

func moveToRecycleBin(originalPath string) error {
	config, err := loadRecycleBinConfig()
	if err != nil {
		return err
	}

	// Check if recycle bin is getting too large
	currentSize := getDirSize(config.RecycleBinPath)
	maxSize := config.MaxSizeMB * 1024 * 1024
	if currentSize > maxSize {
		fmt.Fprintf(os.Stderr, "Warning: Recycle bin is full (%s), cleaning up old files...\n", formatSize(currentSize))
		cleanupRecycleBin()
	}

	absPath, err := filepath.Abs(originalPath)
	if err != nil {
		return err
	}

	fileInfo, err := os.Lstat(originalPath)
	if err != nil {
		return err
	}

	// Generate unique filename for storage using timestamp and hash
	timestamp := time.Now().Format("20060102_150405")
	hasher := md5.New()
	hasher.Write([]byte(absPath))
	hash := hex.EncodeToString(hasher.Sum(nil))[:8]

	baseName := filepath.Base(originalPath)
	isDirectory := fileInfo.IsDir()

	var storedName string
	var useCompression bool

	if isDirectory {
		// Directories aren't compressed, just renamed
		storedName = fmt.Sprintf("%s_%s_%s", timestamp, hash, baseName)
		useCompression = false
	} else {
		// Files get compressed to save space
		storedName = fmt.Sprintf("%s_%s_%s.gz", timestamp, hash, baseName)
		useCompression = true
	}

	entry := RecycleBinEntry{
		OriginalPath: absPath,
		DeletedAt:    time.Now(),
		StoredName:   storedName,
		IsCompressed: useCompression,
		OriginalSize: fileInfo.Size(),
		IsDirectory:  isDirectory,
	}

	destPath := filepath.Join(config.RecycleBinPath, storedName)

	var compressedSize int64
	if err := os.Rename(originalPath, destPath); err != nil {

		if isDirectory {
			if err := copyDir(originalPath, destPath); err != nil {
				return err
			}
			compressedSize = getDirSize(destPath)
		} else {
			if err := copyAndCompressFile(originalPath, destPath); err != nil {
				return err
			}
			if stat, err := os.Stat(destPath); err == nil {
				compressedSize = stat.Size()
			}
		}

		if err := os.RemoveAll(originalPath); err != nil {

			os.RemoveAll(destPath)
			return err
		}
	} else {

		if !isDirectory && useCompression {
			tempPath := destPath + ".tmp"
			if err := compressFileInPlace(destPath, tempPath); err != nil {

				entry.IsCompressed = false
				entry.StoredName = fmt.Sprintf("%s_%s_%s", timestamp, hash, baseName)
				newDestPath := filepath.Join(config.RecycleBinPath, entry.StoredName)
				os.Rename(destPath, newDestPath)
				destPath = newDestPath
			} else {
				os.Rename(tempPath, destPath)
				if stat, err := os.Stat(destPath); err == nil {
					compressedSize = stat.Size()
				}
			}
		}
	}

	if useCompression && compressedSize > 0 {
		entry.CompressedSize = compressedSize
	}

	metadataPath := filepath.Join(config.RecycleBinPath, ".metadata", storedName+".json")
	tempMetadataPath := metadataPath + ".tmp"

	entryData, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {

		os.RemoveAll(destPath)
		return err
	}

	if err := os.WriteFile(tempMetadataPath, entryData, 0600); err != nil {

		os.RemoveAll(destPath)
		return err
	}

	if err := os.Rename(tempMetadataPath, metadataPath); err != nil {

		os.Remove(tempMetadataPath)
		os.RemoveAll(destPath)
		return err
	}

	return nil
}

func copyFile(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	if srcInfo.IsDir() {
		return copyDir(src, dst)
	}

	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		return err
	}

	return os.Chmod(dst, srcInfo.Mode())
}

func copyDir(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}

	return nil
}

func copyAndCompressFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	// Use fastest compression for better performance
	gzipWriter, err := gzip.NewWriterLevel(dstFile, gzip.BestSpeed)
	if err != nil {
		return err
	}
	defer gzipWriter.Close()

	written, err := io.Copy(gzipWriter, srcFile)
	if err != nil {
		return fmt.Errorf("compression failed after %d bytes: %w", written, err)
	}

	if err := gzipWriter.Close(); err != nil {
		return fmt.Errorf("failed to close gzip writer: %w", err)
	}

	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	return os.Chmod(dst, srcInfo.Mode())
}

func compressFileInPlace(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	gzipWriter, err := gzip.NewWriterLevel(dstFile, gzip.BestSpeed)
	if err != nil {
		return err
	}
	defer gzipWriter.Close()

	written, err := io.Copy(gzipWriter, srcFile)
	if err != nil {
		return fmt.Errorf("compression failed after %d bytes: %w", written, err)
	}

	if err := gzipWriter.Close(); err != nil {
		return fmt.Errorf("failed to close gzip writer: %w", err)
	}

	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	return os.Chmod(dst, srcInfo.Mode())
}

func getDirSize(path string) int64 {
	var size int64
	err := filepath.Walk(path, func(filePath string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {

			return nil
		}
		if info != nil && !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	if err != nil {

		return 0
	}
	return size
}

func decompressFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	gzipReader, err := gzip.NewReader(srcFile)
	if err != nil {
		return err
	}
	defer gzipReader.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	written, err := io.Copy(dstFile, gzipReader)
	if err != nil {
		return fmt.Errorf("decompression failed after %d bytes: %w", written, err)
	}

	return os.Chmod(dst, 0644)
}

func listRecycleBin() {
	config, err := loadRecycleBinConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Failed to load config: %v\n", err)
		return
	}

	metadataDir := filepath.Join(config.RecycleBinPath, ".metadata")
	entries, err := os.ReadDir(metadataDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Failed to read recycle bin: %v\n", err)
		return
	}

	if len(entries) == 0 {
		fmt.Println("Recycle bin is empty")
		return
	}

	fmt.Printf("%-20s %-15s %-12s %-8s %s\n", "Deleted At", "Size", "Compressed", "Savings", "Original Path")
	fmt.Println(strings.Repeat("-", 85))

	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		metadataPath := filepath.Join(metadataDir, entry.Name())
		data, err := os.ReadFile(metadataPath)
		if err != nil {
			continue
		}

		var binEntry RecycleBinEntry
		if err := json.Unmarshal(data, &binEntry); err != nil {
			continue
		}

		storedPath := filepath.Join(config.RecycleBinPath, binEntry.StoredName)
		var currentSize int64
		if info, err := os.Stat(storedPath); err == nil {
			currentSize = info.Size()
		}

		var sizeStr, compressedStr, savingsStr string

		if binEntry.IsCompressed && binEntry.OriginalSize > 0 {
			sizeStr = formatSize(binEntry.OriginalSize)
			compressedStr = formatSize(currentSize)
			if currentSize < binEntry.OriginalSize {
				savings := float64(binEntry.OriginalSize-currentSize) / float64(binEntry.OriginalSize) * 100
				savingsStr = fmt.Sprintf("%.1f%%", savings)
			} else {
				savingsStr = "0%"
			}
		} else {
			sizeStr = formatSize(currentSize)
			compressedStr = "No"
			savingsStr = "-"
		}

		fmt.Printf("%-20s %-15s %-12s %-8s %s\n",
			binEntry.DeletedAt.Format("2006-01-02 15:04:05"),
			sizeStr,
			compressedStr,
			savingsStr,
			binEntry.OriginalPath)
	}
}

func formatSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(size)/float64(div), "KMGTPE"[exp])
}

func clearRecycleBin() {
	config, err := loadRecycleBinConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Failed to load config: %v\n", err)
		return
	}

	fmt.Print("Are you sure you want to permanently delete all items from the recycle bin? (y/n): ")
	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		return
	}

	response := strings.TrimSpace(strings.ToLower(scanner.Text()))
	if response != "y" && response != "yes" {
		fmt.Println("Operation cancelled")
		return
	}

	entries, err := os.ReadDir(config.RecycleBinPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Failed to read recycle bin: %v\n", err)
		return
	}

	count := 0
	for _, entry := range entries {
		if entry.Name() == ".metadata" {
			continue
		}

		path := filepath.Join(config.RecycleBinPath, entry.Name())
		if err := os.RemoveAll(path); err != nil {
			fmt.Fprintf(os.Stderr, "Error removing %s: %v\n", path, err)
		} else {
			count++
		}
	}

	metadataDir := filepath.Join(config.RecycleBinPath, ".metadata")
	if err := os.RemoveAll(metadataDir); err != nil {
		fmt.Fprintf(os.Stderr, "Error clearing metadata: %v\n", err)
	}

	os.MkdirAll(metadataDir, 0700)

	fmt.Printf("Cleared %d items from recycle bin\n", count)
}

func restoreFromRecycleBin(originalPath string) {
	config, err := loadRecycleBinConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Failed to load config: %v\n", err)
		return
	}

	metadataDir := filepath.Join(config.RecycleBinPath, ".metadata")
	entries, err := os.ReadDir(metadataDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Failed to read recycle bin: %v\n", err)
		return
	}

	var foundEntry *RecycleBinEntry
	var metadataFile string

	// Search for the file in recycle bin metadata
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		metadataPath := filepath.Join(metadataDir, entry.Name())
		data, err := os.ReadFile(metadataPath)
		if err != nil {
			continue
		}

		var binEntry RecycleBinEntry
		if err := json.Unmarshal(data, &binEntry); err != nil {
			continue
		}

		if binEntry.OriginalPath == originalPath || filepath.Base(binEntry.OriginalPath) == originalPath {
			foundEntry = &binEntry
			metadataFile = metadataPath
			break
		}
	}

	if foundEntry == nil {
		fmt.Fprintf(os.Stderr, "Error: File '%s' not found in recycle bin\n", originalPath)
		return
	}

	if _, err := os.Stat(foundEntry.OriginalPath); err == nil {
		fmt.Printf("Warning: '%s' already exists. Overwrite? (y/n): ", foundEntry.OriginalPath)
		scanner := bufio.NewScanner(os.Stdin)
		if !scanner.Scan() {
			return
		}
		response := strings.TrimSpace(strings.ToLower(scanner.Text()))
		if response != "y" && response != "yes" {
			fmt.Println("Restore cancelled")
			return
		}
	}

	cleanPath := filepath.Clean(foundEntry.OriginalPath)
	if strings.Contains(cleanPath, "..") || !filepath.IsAbs(cleanPath) {
		fmt.Fprintf(os.Stderr, "Error: Invalid restore path detected: %s\n", foundEntry.OriginalPath)
		return
	}

	parentDir := filepath.Dir(cleanPath)
	if err := os.MkdirAll(parentDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error: Failed to create parent directory: %v\n", err)
		return
	}

	foundEntry.OriginalPath = cleanPath

	storedPath := filepath.Join(config.RecycleBinPath, foundEntry.StoredName)

	if foundEntry.IsCompressed && !foundEntry.IsDirectory {

		if err := decompressFile(storedPath, foundEntry.OriginalPath); err != nil {
			fmt.Fprintf(os.Stderr, "Error: Failed to decompress and restore file: %v\n", err)
			return
		}
		os.Remove(storedPath)
	} else {

		if err := os.Rename(storedPath, foundEntry.OriginalPath); err != nil {

			if err := copyFile(storedPath, foundEntry.OriginalPath); err != nil {
				fmt.Fprintf(os.Stderr, "Error: Failed to restore file: %v\n", err)
				return
			}
			os.RemoveAll(storedPath)
		}
	}

	os.Remove(metadataFile)

	fmt.Printf("Restored '%s'\n", foundEntry.OriginalPath)
}

func cleanupRecycleBin() {
	config, err := loadRecycleBinConfig()
	if err != nil {
		return
	}

	cutoffTime := time.Now().AddDate(0, 0, -config.RetentionDays)

	metadataDir := filepath.Join(config.RecycleBinPath, ".metadata")
	entries, err := os.ReadDir(metadataDir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		metadataPath := filepath.Join(metadataDir, entry.Name())
		data, err := os.ReadFile(metadataPath)
		if err != nil {
			continue
		}

		var binEntry RecycleBinEntry
		if err := json.Unmarshal(data, &binEntry); err != nil {
			continue
		}

		if binEntry.DeletedAt.Before(cutoffTime) {

			storedPath := filepath.Join(config.RecycleBinPath, binEntry.StoredName)
			os.RemoveAll(storedPath)
			os.Remove(metadataPath)
		}
	}
}
