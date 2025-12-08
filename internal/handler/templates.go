package handler

import (
	"html/template"
	"log"
)

// TemplateFuncs returns the custom template functions used across all templates
func TemplateFuncs() template.FuncMap {
	return template.FuncMap{
		// mul multiplies two numbers
		"mul": func(a, b float64) float64 {
			return a * b
		},
		// list creates a slice from arguments
		"list": func(args ...interface{}) []interface{} {
			return args
		},
		// seq generates a sequence of integers from start to end (inclusive)
		"seq": func(start, end int) []int {
			result := make([]int, 0, end-start+1)
			for i := start; i <= end; i++ {
				result = append(result, i)
			}
			return result
		},
		// add adds two integers
		"add": func(a, b int) int {
			return a + b
		},
		// sub subtracts two integers
		"sub": func(a, b int) int {
			return a - b
		},
	}
}

// LoadTemplates loads all HTML templates with custom functions
func LoadTemplates() *template.Template {
	tmpl := template.New("").Funcs(TemplateFuncs())
	tmpl, err := tmpl.ParseGlob("templates/*.html")
	if err != nil {
		log.Printf("Warning: failed to load templates: %v", err)
		return template.New("empty").Funcs(TemplateFuncs())
	}
	return tmpl
}
