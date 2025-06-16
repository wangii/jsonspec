package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"reflect"
	"text/template"

	"github.com/sashabaranov/go-openai"
	"github.com/wangii/jsonspec"
)

func injectSpec[T any](param any) error {
	if reflect.TypeOf(param).Kind() != reflect.Ptr {
		return fmt.Errorf("参数必须是指针")
	}

	if reflect.TypeOf(param).Elem().Kind() != reflect.Struct {
		return fmt.Errorf("参数必须是结构体指针")
	}

	bsspec, err := jsonspec.SpecMarshal(reflect.TypeOf((*T)(nil)).Elem(), "", "  ")
	if err != nil {
		return fmt.Errorf("生成模型OutSpec参数失败: %v", err)
	}

	p := reflect.ValueOf(param).Elem()
	f := p.FieldByName("OutSpec")

	if f.IsValid() == false || f.Kind() != reflect.String || !f.CanSet() {
		return fmt.Errorf("参数缺少OutSpec字段")
	}

	spec := "```json\n" + string(bsspec) + "\n```"
	f.Set(reflect.ValueOf(spec))

	return nil
}

func llmCall[T any](sys string, tpl *template.Template, param any) (*T, error) {

	err := injectSpec[T](param)
	if err != nil {
		return nil, err
	}

	up := bytes.NewBufferString("")
	_ = tpl.Execute(up, param)

	messages := []openai.ChatCompletionMessage{
		{
			Role:    openai.ChatMessageRoleSystem,
			Content: sys,
		},
		{
			Role:    openai.ChatMessageRoleUser,
			Content: up.String(),
		},
	}

	log.Printf("提示词为:%v", up.String())

	// 调用模型接口
	// modelOutput, err := llm.QwenChat(messages, "qwen-max-latest")
	modelOutput, err := messages[1].Content, nil
	if err != nil {
		return nil, fmt.Errorf("模型处理失败: %v", err)
	}

	// 可选：校验返回结果是否为合法JSON
	structModelOutput, err := extractJSON(modelOutput)
	log.Printf("模型返回检验后的json输出:%v", structModelOutput)
	if err != nil {
		return nil, fmt.Errorf("模型生成json格式不符合要求: %v", err)
	}

	var result T
	err = json.Unmarshal([]byte(structModelOutput), &result)
	if err != nil {
		return nil, fmt.Errorf("解析 JSON 出错: %v", err)
	}

	return &result, nil
}
