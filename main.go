package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os/exec"
	"strings"
)

func executeCommand(command string, args ...string) string {
	cmd := exec.Command(command, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("Command execution failed: %s\nCommand: %s %s\nOutput: %s\n", err, command, strings.Join(args, " "), out)
		return ""
	}
	return strings.TrimSpace(string(out))
}

func cloneRepo(repoURL, destPath string) bool {
	_ = executeCommand("git", "clone", repoURL, destPath)
	return true
}
func gitHistory(filename string) string {
	cmdStr := fmt.Sprintf("git -C ./tmp_repo log -- %s | grep 'Date:'", filename)
	return executeCommand("sh", "-c", cmdStr)
}

func firstCommit(filename string) string {
	history := gitHistory(filename)
	allCommits := strings.Split(history, "\n")
	if len(allCommits) > 0 {
		return parseDate(allCommits[len(allCommits)-1])
	}
	return ""
}

func lastCommit(filename string) string {
	history := gitHistory(filename)
	allCommits := strings.Split(history, "\n")
	if len(allCommits) > 0 {
		return parseDate(allCommits[0])
	}
	return ""
}

func parseDate(dateLine string) string {
	parts := strings.Fields(dateLine)
	if len(parts) >= 6 {
		month := parts[2]
		day := parts[3]
		year := parts[5]
		return month + " " + day + " " + year
	}
	return ""
}

func numberOfCommits(filename string) string {
	history := gitHistory(filename)
	commitCount := len(strings.Split(history, "\n"))
	return fmt.Sprintf("%d", commitCount)
}

func linesOfCode(filename string) string {
	return strings.Fields(executeCommand("wc", "-l", filename))[0]
}

func csvFor(extension string, directory string) string {
	files := strings.Split(executeCommand("find", directory, "-iname", "*."+extension), "\n")
	var result []string

	var cloneDir = "./tmp_repo"

	for _, filename := range files {
		if filename != "" {
			csvFilename := strings.TrimPrefix(filename, cloneDir+"/")

			lc := linesOfCode(filename)
			nc := numberOfCommits(csvFilename)
			fc := firstCommit(csvFilename)
			lcmt := lastCommit(csvFilename)

			line := fmt.Sprintf("%s,%s,%s,%s,%s", csvFilename, lc, nc, fc, lcmt)
			result = append(result, line)
		}
	}

	return strings.Join(result, "\n")
}

func csvToHTMLTable(csvData string) string {
	lines := strings.Split(csvData, "\n")
	if len(lines) == 0 {
		return ""
	}

	var builder strings.Builder
	builder.WriteString("<table border='1'>")

	headerCells := strings.Split(lines[0], ",")
	builder.WriteString("<thead><tr>")
	for _, cell := range headerCells {
		builder.WriteString(fmt.Sprintf("<th>%s</th>", cell))
	}
	builder.WriteString("</tr></thead><tbody>")

	for _, line := range lines[1:] {
		cells := strings.Split(line, ",")
		builder.WriteString("<tr>")
		for _, cell := range cells {
			builder.WriteString(fmt.Sprintf("<td>%s</td>", cell))
		}
		builder.WriteString("</tr>")
	}
	builder.WriteString("</tbody></table>")

	return builder.String()
}

func handleRequest(w http.ResponseWriter, r *http.Request) {
	repoURL := r.URL.Query().Get("repo")
	extensions := r.URL.Query()["ext"]
	cloneDir := "./tmp_repo"

	if repoURL == "" || len(extensions) == 0 {
		http.Error(w, "Please provide a repository URL and at least one extension.", http.StatusBadRequest)
		return
	}

	success := cloneRepo(repoURL, cloneDir)
	if !success {
		http.Error(w, "Failed to clone repository.", http.StatusInternalServerError)
		return
	}

	header := "filename,lines of code,number of commits,date of first commit,date of last commit\n"
	var results []string
	for _, ext := range extensions {
		results = append(results, csvFor(ext, cloneDir))
	}

	// Construct the final CSV data
	csvData := header + strings.Join(results, "\n")

	err := ioutil.WriteFile("output.csv", []byte(csvData), 0644)
	if err != nil {
		http.Error(w, "Failed to save CSV data to file.", http.StatusInternalServerError)
		return
	}

	_ = executeCommand("rm", "-rf", cloneDir)

	const cssStyles = `
<style>
    table {
        border-collapse: collapse;
        width: 80%;
        margin: 50px auto;
        font-family: Arial, sans-serif;
    }
    th, td {
        border: 1px solid #ddd;
        padding: 8px;
        text-align: left;
    }
    th {
        background-color: #f2f2f2;
    }
    tr:hover {
        background-color: #f5f5f5;
    }
    th, td {
        padding: 15px;
        text-align: left;
    }
</style>
`

	htmlTable := csvToHTMLTable(csvData)
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprint(w, cssStyles+htmlTable)
}

func main() {
	http.HandleFunc("/", handleRequest)
	log.Fatal(http.ListenAndServe(":8080", nil))
}
