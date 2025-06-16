package main

import (
	"fmt"
	"log"
	"sort"
	"strings"
	"text/template"
	"text/template/parse"
)

// FieldInfo holds information about a template field
type FieldInfo struct {
	Name     string
	Type     string
	IsNested bool
	Parent   string // For nested fields, the parent struct name
}

// StructDefinition represents a complete struct with its fields and nested structs
type StructDefinition struct {
	Name          string
	Fields        []FieldInfo
	NestedStructs map[string]*StructDefinition
}

// ExtractTemplateFields extracts field requirements from a Go template string
// Returns both flat fields and nested struct information
func ExtractTemplateFields(templateStr string) (*StructDefinition, error) {
	// Parse the template
	tmpl, err := template.New("extract").Parse(templateStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse template: %w", err)
	}

	// Get the root node
	root := tmpl.Root
	if root == nil {
		return nil, fmt.Errorf("template has no root node")
	}

	fieldPaths := make(map[string]bool)

	// Walk through all nodes to find field references
	walkNodes(root.Nodes, fieldPaths, templateStr, "")

	// Build struct hierarchy from field paths
	rootStruct := buildStructHierarchy(fieldPaths, templateStr)

	return rootStruct, nil
}

// walkNodes recursively walks through template nodes to find field references
func walkNodes(nodes []parse.Node, fieldPaths map[string]bool, template string, currentContext string) {
	for _, node := range nodes {
		switch n := node.(type) {
		case *parse.ActionNode:
			// Handle action nodes like {{.Field}}
			walkPipe(n.Pipe, fieldPaths, template, currentContext)
		case *parse.IfNode:
			// Handle if conditions
			walkPipe(n.Pipe, fieldPaths, template, currentContext)
			walkNodes(n.List.Nodes, fieldPaths, template, currentContext)
			if n.ElseList != nil {
				walkNodes(n.ElseList.Nodes, fieldPaths, template, currentContext)
			}
		case *parse.RangeNode:
			// Handle range loops - the context changes inside range
			rangeContext := extractRangeContext(n.Pipe)
			walkPipe(n.Pipe, fieldPaths, template, currentContext)
			walkNodes(n.List.Nodes, fieldPaths, template, rangeContext)
			if n.ElseList != nil {
				walkNodes(n.ElseList.Nodes, fieldPaths, template, currentContext)
			}
		case *parse.WithNode:
			// Handle with statements - the context changes inside with
			withContext := extractWithContext(n.Pipe)
			walkPipe(n.Pipe, fieldPaths, template, currentContext)
			walkNodes(n.List.Nodes, fieldPaths, template, withContext)
			if n.ElseList != nil {
				walkNodes(n.ElseList.Nodes, fieldPaths, template, currentContext)
			}
		case *parse.TemplateNode:
			// Handle template inclusions
			walkPipe(n.Pipe, fieldPaths, template, currentContext)
		}
	}
}

// extractRangeContext extracts the field path from a range statement
func extractRangeContext(pipe *parse.PipeNode) string {
	if pipe == nil || len(pipe.Cmds) == 0 {
		return ""
	}

	cmd := pipe.Cmds[0]
	if len(cmd.Args) == 0 {
		return ""
	}

	if field, ok := cmd.Args[0].(*parse.FieldNode); ok {
		return strings.Join(field.Ident, ".")
	}

	return ""
}

// extractWithContext extracts the field path from a with statement
func extractWithContext(pipe *parse.PipeNode) string {
	if pipe == nil || len(pipe.Cmds) == 0 {
		return ""
	}

	cmd := pipe.Cmds[0]
	if len(cmd.Args) == 0 {
		return ""
	}

	if field, ok := cmd.Args[0].(*parse.FieldNode); ok {
		return strings.Join(field.Ident, ".")
	}

	return ""
}

// walkPipe walks through a pipe to find field references
func walkPipe(pipe *parse.PipeNode, fieldPaths map[string]bool, template string, currentContext string) {
	if pipe == nil {
		return
	}

	for _, cmd := range pipe.Cmds {
		walkCommand(cmd, fieldPaths, template, currentContext)
	}
}

// walkCommand walks through a command to find field references
func walkCommand(cmd *parse.CommandNode, fieldPaths map[string]bool, template string, currentContext string) {
	for _, arg := range cmd.Args {
		switch n := arg.(type) {
		case *parse.FieldNode:
			// This is a field reference like .Field or .User.Name
			if len(n.Ident) > 0 {
				var fullPath string
				if currentContext != "" {
					// Inside a with/range block, prepend the context
					fullPath = currentContext + "." + strings.Join(n.Ident, ".")
				} else {
					fullPath = strings.Join(n.Ident, ".")
				}
				fieldPaths[fullPath] = true
			}
		case *parse.VariableNode:
			// Handle variable references like $var
			for _, ident := range n.Ident {
				if ident != "" && ident != "$" {
					fieldPaths[ident] = true
				}
			}
		case *parse.PipeNode:
			// Handle nested pipes
			walkPipe(n, fieldPaths, template, currentContext)
		}
	}
}

// buildStructHierarchy builds nested struct definitions from field paths
func buildStructHierarchy(fieldPaths map[string]bool, template string) *StructDefinition {
	rootStruct := &StructDefinition{
		Name:          "RootData",
		Fields:        []FieldInfo{},
		NestedStructs: make(map[string]*StructDefinition),
	}

	// Group fields by their parent path
	fieldGroups := make(map[string][]string)

	for path := range fieldPaths {
		parts := strings.Split(path, ".")
		if len(parts) == 1 {
			// Top-level field
			fieldGroups[""] = append(fieldGroups[""], path)
		} else {
			// Nested field
			parentPath := strings.Join(parts[:len(parts)-1], ".")
			fieldGroups[parentPath] = append(fieldGroups[parentPath], path)
		}
	}

	// Create struct definitions
	createdStructs := make(map[string]*StructDefinition)

	// Process all field groups
	for parentPath, fields := range fieldGroups {
		var targetStruct *StructDefinition

		if parentPath == "" {
			// Root level fields
			targetStruct = rootStruct
		} else {
			// Create or get nested struct
			structName := getStructNameFromPath(parentPath)
			if existing, exists := createdStructs[structName]; exists {
				targetStruct = existing
			} else {
				targetStruct = &StructDefinition{
					Name:          structName,
					Fields:        []FieldInfo{},
					NestedStructs: make(map[string]*StructDefinition),
				}
				createdStructs[structName] = targetStruct
			}
		}

		// Add fields to the target struct
		for _, fieldPath := range fields {
			parts := strings.Split(fieldPath, ".")
			fieldName := parts[len(parts)-1]

			if fieldName != "" {
				structFieldName := strings.Title(fieldName)
				fieldInfo := FieldInfo{
					Name: structFieldName,
					Type: inferFieldType(fieldName),
					// JSONTag:  strings.ToLower(fieldName),
					// Required: true,
					IsNested: len(parts) > 1,
					Parent:   parentPath,
				}

				// Check if this field represents a nested struct
				nestedStructName := getStructNameFromPath(fieldPath)
				if _, hasNestedFields := fieldGroups[fieldPath]; hasNestedFields {
					fieldInfo.Type = nestedStructName
				}

				targetStruct.Fields = append(targetStruct.Fields, fieldInfo)
			}
		}
	}

	// Link nested structs to their parents
	for parentPath, nestedStruct := range createdStructs {
		if parentPath == "" {
			continue
		}

		// Find the parent struct
		parentParts := strings.Split(parentPath, ".")
		if len(parentParts) == 1 {
			// Direct child of root
			rootStruct.NestedStructs[nestedStruct.Name] = nestedStruct
		} else {
			// Find parent in created structs
			parentStructPath := strings.Join(parentParts[:len(parentParts)-1], ".")
			parentStructName := getStructNameFromPath(parentStructPath)
			if parent, exists := createdStructs[parentStructName]; exists {
				parent.NestedStructs[nestedStruct.Name] = nestedStruct
			}
		}
	}

	// Sort fields in all structs
	sortStructFields(rootStruct)

	return rootStruct
}

// getStructNameFromPath generates a struct name from a field path
func getStructNameFromPath(path string) string {
	if path == "" {
		return "RootData"
	}

	parts := strings.Split(path, ".")
	var nameParts []string
	for _, part := range parts {
		if part != "" {
			nameParts = append(nameParts, strings.Title(part))
		}
	}
	return strings.Join(nameParts, "") + "Data"
}

// sortStructFields recursively sorts fields in all structs
func sortStructFields(structDef *StructDefinition) {
	sort.Slice(structDef.Fields, func(i, j int) bool {
		return structDef.Fields[i].Name < structDef.Fields[j].Name
	})

	for _, nestedStruct := range structDef.NestedStructs {
		sortStructFields(nestedStruct)
	}
}

// inferFieldType attempts to infer the Go type based on field name patterns
func inferFieldType(fieldName string) string {
	lower := strings.ToLower(fieldName)

	// Common patterns for different types
	switch {
	case strings.Contains(lower, "id") || strings.HasSuffix(lower, "id"):
		return "int64"
	case strings.Contains(lower, "count") || strings.Contains(lower, "number") || strings.Contains(lower, "num"):
		return "int"
	case strings.Contains(lower, "price") || strings.Contains(lower, "amount") || strings.Contains(lower, "cost"):
		return "float64"
	case strings.Contains(lower, "is") || strings.Contains(lower, "has") || strings.Contains(lower, "enabled"):
		return "bool"
	case strings.Contains(lower, "date") || strings.Contains(lower, "time") || strings.Contains(lower, "created") || strings.Contains(lower, "updated"):
		return "time.Time"
	case strings.Contains(lower, "email"):
		return "string"
	case strings.Contains(lower, "url") || strings.Contains(lower, "link"):
		return "string"
	default:
		return "string"
	}
}

// GenerateAllStructDefinitions creates Go struct definitions for root and all nested structs
func GenerateAllStructDefinitions(rootStruct *StructDefinition, packageName string) string {
	var builder strings.Builder

	// Add package declaration if provided
	if packageName != "" {
		builder.WriteString(fmt.Sprintf("package %s\n\n", packageName))
	}

	// // Check if we need time import
	// needsTime := checkNeedsTimeImport(rootStruct)
	// if needsTime {
	// 	builder.WriteString("import (\n\t\"time\"\n)\n\n")
	// }

	// Generate all struct definitions
	generateStructDefinitions(rootStruct, &builder, make(map[string]bool))

	return builder.String()
}

// // checkNeedsTimeImport recursively checks if any field uses time.Time
// func checkNeedsTimeImport(structDef *StructDefinition) bool {
// 	for _, field := range structDef.Fields {
// 		if field.Type == "time.Time" {
// 			return true
// 		}
// 	}

// 	for _, nestedStruct := range structDef.NestedStructs {
// 		if checkNeedsTimeImport(nestedStruct) {
// 			return true
// 		}
// 	}

// 	return false
// }

// generateStructDefinitions recursively generates struct definitions
func generateStructDefinitions(structDef *StructDefinition, builder *strings.Builder, generated map[string]bool) {
	// Generate nested structs first (dependencies)
	for _, nestedStruct := range structDef.NestedStructs {
		if !generated[nestedStruct.Name] {
			generateStructDefinitions(nestedStruct, builder, generated)
		}
	}

	// Generate current struct if not already generated
	if !generated[structDef.Name] {
		builder.WriteString(fmt.Sprintf("type %s struct {\n", structDef.Name))

		for _, field := range structDef.Fields {
			// Handle slice types for range contexts
			fieldType := field.Type
			if isSliceField(field.Name, structDef.Name) {
				if !strings.HasPrefix(fieldType, "[]") {
					fieldType = "[]" + fieldType
				}
			}

			line := fmt.Sprintf("\t%s %s", field.Name, fieldType)

			// // Add JSON tag
			// if field.JSONTag != "" {
			// 	tag := fmt.Sprintf("`json:\"%s\"", field.JSONTag)
			// 	if field.Required {
			// 		tag += " validate:\"required\""
			// 	}
			// 	tag += "`"
			// 	line += " " + tag
			// }

			builder.WriteString(line + "\n")
		}

		builder.WriteString("}\n\n")
		generated[structDef.Name] = true
	}
}

// isSliceField determines if a field should be a slice based on context
func isSliceField(fieldName, structName string) bool {
	lower := strings.ToLower(fieldName)
	// Common patterns for slice fields
	return strings.HasSuffix(lower, "s") ||
		strings.Contains(lower, "list") ||
		strings.Contains(lower, "items") ||
		strings.Contains(lower, "entries")
}

// TemplateProcessor combines extraction and struct generation
type TemplateProcessor struct {
	DefaultPackage string
}

// ProcessTemplate extracts fields from template and generates struct definitions
func (tp *TemplateProcessor) ProcessTemplate(template, structName string) (string, error) {
	rootStruct, err := ExtractTemplateFields(template)
	if err != nil {
		return "", fmt.Errorf("failed to extract fields: %w", err)
	}

	if len(rootStruct.Fields) == 0 && len(rootStruct.NestedStructs) == 0 {
		return "", fmt.Errorf("no template fields found")
	}

	// Use provided struct name for root
	rootStruct.Name = structName

	structDef := GenerateAllStructDefinitions(rootStruct, tp.DefaultPackage)
	return structDef, nil
}

// Example usage and test function
func main2() {
	// Example Go templates with nested structures
	templates := map[string]string{
		"user_profile": `
			{{with .User}}
			Name: {{.Name}}
			Email: {{.Email}}
			Age: {{.Age}}
			
			{{with .Profile}}
			Bio: {{.Bio}}
			Avatar: {{.AvatarUrl}}
			Created: {{.CreatedAt}}
			{{end}}
			
			{{with .Settings}}
			Theme: {{.Theme}}
			Notifications: {{.EnableNotifications}}
			Privacy: {{.IsPrivate}}
			{{end}}
			{{end}}
		`,
		"order_details": `
			Order #{{.OrderId}} for {{.CustomerName}}
			Total: ${{.TotalAmount}}
			Date: {{.OrderDate}}
			
			{{range .Items}}
			- {{.Name}}: ${{.Price}} (Qty: {{.Quantity}})
			  {{with .Product}}
			  SKU: {{.Sku}}
			  Category: {{.Category}}
			  In Stock: {{.InStock}}
			  {{end}}
			{{end}}
			
			{{with .Shipping}}
			Address: {{.Address}}
			City: {{.City}}
			Zip: {{.ZipCode}}
			Express: {{.IsExpress}}
			{{end}}
			
			{{with .Payment}}
			Method: {{.Method}}
			Last4: {{.CardLast4}}
			Processed: {{.ProcessedAt}}
			{{end}}
		`,
		"company_report": `
			{{.CompanyName}} Quarterly Report
			Generated: {{.GeneratedAt}}
			Quarter: {{.Quarter}}
			
			{{range .Departments}}
			Department: {{.Name}}
			Manager: {{.Manager}}
			
			{{range .Employees}}
			- {{.Name}} ({{.Position}})
			  Salary: ${{.Salary}}
			  Start Date: {{.StartDate}}
			  
			  {{with .Contact}}
			  Email: {{.Email}}
			  Phone: {{.Phone}}
			  {{end}}
			{{end}}
			
			Budget: ${{.Budget}}
			Utilization: {{.BudgetUtilization}}%
			{{end}}
			
			Total Revenue: ${{.TotalRevenue}}
			Is Profitable: {{.IsProfitable}}
		`,
		"eg": strIdentifyErr,
	}

	processor := &TemplateProcessor{
		DefaultPackage: "models",
	}

	log.Println("=== Template Field Extraction with Nested Structs ===")

	for name, templateStr := range templates {
		fmt.Printf("Template: %s\n", name)
		fmt.Printf("Template content:\n%s\n", templateStr)

		// Extract fields and generate structs
		structName := strings.Title(name) + "Data"
		structDef, err := processor.ProcessTemplate(templateStr, structName)
		if err != nil {
			fmt.Printf("Error: %v\n\n", err)
			continue
		}

		fmt.Printf("Generated structs:\n%s", structDef)
		fmt.Println(strings.Repeat("-", 80))
	}
}
