package main

import (
	_ "embed"
	"fmt"
	"log"
	"regexp"
	"text/template"
)

var (
	//go:embed prompts/identify_err.md
	strIdentifyErr string
	tplIdentifyErr *template.Template
)

func init() {
	tplIdentifyErr = template.Must(template.New("identify_err").Parse(strIdentifyErr))
}

func extractJSON(s string) (string, error) {
	re := regexp.MustCompile("(?s)```json\\s*(\\{.*?\\})\\s*```")
	match := re.FindStringSubmatch(s)
	if len(match) >= 2 {
		jsonStr := match[1]
		return jsonStr, nil
	}
	return "", fmt.Errorf("未找到 JSON 代码块")
}

func main() {
	type OutSpec struct {
		Errors []struct {
			Major       string  `json:"主要维度" spec:"\"主要维度\""`
			Minor       string  `json:"二级维度" spec:"\"二级维度\""`
			Probability float64 `json:"主要原因概率" spec:"0.0-1.0"`
		} `json:"错误类型"`
	}

	res, err := llmCall[OutSpec]("", tplIdentifyErr, &struct {
		OutSpec         string
		QuestionContent string
		SubQuestion     string
		StudentAnswer   string
		ErrorReason     string
	}{
		QuestionContent: "请从下面错误信息中提取主要维度、二级维度以及主要原因概率，并给出相应的错误类型。",
		SubQuestion:     "请从下面错误信息中提取主要维度、二级维度以及主要原因概率，并给出相应的错误类型。",
		StudentAnswer:   "请从下面错误信息中提取主要维度、二级维度以及主要原因概率，并给出相应的错误类型。",
		ErrorReason:     "请从下面错误信息中提取主要",
	})

	log.Print(res, err)
}
