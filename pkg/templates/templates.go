package templates

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	
	"github.com/pdxmph/imgupv2/pkg/backends"
)

// Variables holds all the available template variables
type Variables struct {
	// Basic info
	PhotoID  string
	URL      string // Photo page URL
	ImageURL string // Direct image URL
	EditURL  string // Flickr edit URL
	Filename string // Original filename
	
	// Metadata
	Title       string
	Description string
	Alt         string
	Tags        []string
}

var (
	// Match %variable% or %var1|var2|var3% (with pipes inside the percent signs)
	// This pattern matches content between % signs, which may contain pipes
	templatePattern = regexp.MustCompile(`%([^%]+)%`)
)

// Process renders a template with the given variables
func Process(template string, vars Variables) string {
	result := templatePattern.ReplaceAllStringFunc(template, func(match string) string {
		// Remove the % delimiters
		content := strings.Trim(match, "%")
		
		if os.Getenv("IMGUP_DEBUG") != "" {
			fmt.Fprintf(os.Stderr, "DEBUG: Template processing match: %s\n", match)
		}
		
		// Check if it's a fallback chain
		if strings.Contains(content, "|") {
			// Handle fallback chain
			parts := strings.Split(content, "|")
			if os.Getenv("IMGUP_DEBUG") != "" {
				fmt.Fprintf(os.Stderr, "DEBUG: Fallback chain parts: %v\n", parts)
			}
			for _, part := range parts {
				value := getVariable(strings.TrimSpace(part), vars)
				if os.Getenv("IMGUP_DEBUG") != "" {
					fmt.Fprintf(os.Stderr, "DEBUG: Trying '%s' = '%s'\n", part, value)
				}
				if value != "" {
					return value
				}
			}
			return ""
		}
		
		// Single variable
		value := getVariable(content, vars)
		if os.Getenv("IMGUP_DEBUG") != "" {
			fmt.Fprintf(os.Stderr, "DEBUG: Single variable '%s' = '%s'\n", content, value)
		}
		return value
	})
	
	return result
}

// getVariable returns the value of a single variable
func getVariable(name string, vars Variables) string {
	switch name {
	case "photo_id":
		return vars.PhotoID
	case "url":
		return vars.URL
	case "image_url":
		return vars.ImageURL
	case "edit_url":
		return vars.EditURL
	case "filename":
		return vars.Filename
	case "title":
		return vars.Title
	case "description":
		return vars.Description
	case "alt":
		return vars.Alt
	case "tags":
		return strings.Join(vars.Tags, ", ")
	default:
		return ""
	}
}

// BuildVariables creates template variables from upload result and metadata
func BuildVariables(result *backends.UploadResult, imagePath, title, description, alt string, tags []string) Variables {
	filename := filepath.Base(imagePath)
	filenameNoExt := strings.TrimSuffix(filename, filepath.Ext(filename))
	
	// Build edit URL
	editURL := "https://www.flickr.com/photos/upload/edit/?ids=" + result.PhotoID
	
	return Variables{
		PhotoID:     result.PhotoID,
		URL:         result.URL,
		ImageURL:    result.ImageURL,
		EditURL:     editURL,
		Filename:    filenameNoExt,
		Title:       title,
		Description: description,
		Alt:         alt,
		Tags:        tags,
	}
}
