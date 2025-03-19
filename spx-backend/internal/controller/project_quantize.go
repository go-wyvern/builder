package controller

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/goplus/builder/spx-backend/internal/log"
	"github.com/goplus/builder/spx-backend/internal/model"
)

// ProjectQuantizationRequest holds the request to quantize a specific project
type ProjectQuantizationRequest struct {
	ProjectID uint `json:"projectId"`
}

// QuantizeProjectResponse is the response for a project quantization request
type QuantizeProjectResponse struct {
	Success      bool   `json:"success"`
	ProjectID    uint   `json:"projectId"`
	SnippetCount int    `json:"snippetCount"`
	Message      string `json:"message"`
}

// QuantizeProject processes a specific project for knowledge base incorporation
func (ctrl *Controller) QuantizeProject(ctx context.Context, req *ProjectQuantizationRequest) (*QuantizeProjectResponse, error) {
	// Retrieve the project by ID
	var project model.Project
	if err := ctrl.db.WithContext(ctx).
		Preload("Owner").
		Preload("LatestRelease").
		Where("id = ?", req.ProjectID).
		First(&project).Error; err != nil {
		return &QuantizeProjectResponse{
			Success:   false,
			ProjectID: req.ProjectID,
			Message:   fmt.Sprintf("Failed to find project: %v", err),
		}, nil
	}

	// Check if project is public
	if project.Visibility != model.VisibilityPublic {
		return &QuantizeProjectResponse{
			Success:   false,
			ProjectID: req.ProjectID,
			Message:   "Project is not public, skipping quantization",
		}, nil
	}

	// Ensure project has a release
	if project.LatestRelease == nil {
		return &QuantizeProjectResponse{
			Success:   false,
			ProjectID: req.ProjectID,
			Message:   "Project has no latest release to quantize",
		}, nil
	}

	// Get the latest release with all data
	var release model.ProjectRelease
	if err := ctrl.db.WithContext(ctx).
		Where("id = ?", project.LatestRelease.ID).
		First(&release).Error; err != nil {
		return &QuantizeProjectResponse{
			Success:   false,
			ProjectID: req.ProjectID,
			Message:   fmt.Sprintf("Failed to load latest release: %v", err),
		}, nil
	}

	// Process quantization
	snippetCount, err := ctrl.processProjectQuantization(ctx, &project, &release)
	if err != nil {
		return &QuantizeProjectResponse{
			Success:   false,
			ProjectID: req.ProjectID,
			Message:   fmt.Sprintf("Quantization failed: %v", err),
		}, nil
	}

	return &QuantizeProjectResponse{
		Success:      true,
		ProjectID:    req.ProjectID,
		SnippetCount: snippetCount,
		Message:      "Project successfully quantized",
	}, nil
}

// processProjectQuantization handles the actual quantization process for a specific project
func (ctrl *Controller) processProjectQuantization(ctx context.Context, project *model.Project, release *model.ProjectRelease) (int, error) {
	logger := log.GetLogger()
	projectFullName := fmt.Sprintf("%s/%s", project.Owner.Username, project.Name)

	// Extract code snippets from project files
	snippets, err := ctrl.extractCodeSnippets(ctx, projectFullName, *release)
	if err != nil {
		return 0, fmt.Errorf("failed to extract code snippets: %w", err)
	}

	// Process each code snippet
	snippetCount := 0
	for _, snippet := range snippets {
		// Calculate hash for the snippet
		snippetHash := calculateSnippetHash(snippet)

		// Check if we already have this snippet
		var existingSnippet model.ProjectCodeSnippet
		err = ctrl.db.Where("project_id = ? AND code_hash = ?", project.ID, snippetHash).First(&existingSnippet).Error
		if err == nil {
			// Update existing snippet with new release ID
			if err := ctrl.db.Model(&existingSnippet).Updates(map[string]interface{}{
				"release_id": release.ID,
				"updated_at": time.Now(),
			}).Error; err != nil {
				return snippetCount, fmt.Errorf("failed to update code snippet: %w", err)
			}
			snippetCount++
			continue
		} else if err != gorm.ErrRecordNotFound {
			return snippetCount, err
		}

		// Generate embedding for the snippet
		embedding, err := ctrl.embedded.GetEmbedding(ctx, snippet.Content)
		if err != nil {
			// Log error but continue with other snippets
			logger.Warn("Failed to generate embedding", "error", err, "projectID", project.ID, "path", snippet.FilePath)
			continue
		}

		// Add the embedding to Faiss
		err = ctrl.knowledgeBase.AddEmbedding(embedding)
		if err != nil {
			return snippetCount, fmt.Errorf("failed to add embedding to vector database: %w", err)
		}

		// Store snippet metadata
		newSnippet := model.ProjectCodeSnippet{
			ProjectID: project.ID,
			ReleaseID: release.ID,
			FilePath:  snippet.FilePath,
			CodeHash:  snippetHash,
			Context:   snippet.Context,
		}

		if err := ctrl.db.Create(&newSnippet).Error; err != nil {
			return snippetCount, fmt.Errorf("failed to store code snippet: %w", err)
		}

		// Add to the knowledge base metadata map
		ctrl.knowledgeBase.AddCodeMeta(snippetHash, model.CodeMeta{
			ProjectID: fmt.Sprintf("%d", project.ID),
			FilePath:  snippet.FilePath,
			CodeHash:  snippetHash,
			Embedding: embedding,
			Context:   snippet.Context,
		})

		snippetCount++
	}

	// Extract and process SPX API patterns
	apiPatterns, err := ctrl.extractSPXAPIPatterns(ctx, *release)
	if err != nil {
		return snippetCount, fmt.Errorf("failed to extract API patterns: %w", err)
	}

	// Store API record in knowledge base
	for apiName, record := range apiPatterns {
		ctrl.knowledgeBase.AddAPIRecord(apiName, record)
	}

	// Save knowledge base changes to disk
	if err := ctrl.SaveKnowledgeBase(); err != nil {
		return snippetCount, fmt.Errorf("failed to save knowledge base: %w", err)
	}

	return snippetCount, nil
}

// CodeSnippet represents a code snippet extracted from a project
type CodeSnippet struct {
	FilePath string
	Content  string
	Context  string
}

// extractCodeSnippets extracts code snippets from a project release
func (ctrl *Controller) extractCodeSnippets(ctx context.Context, projectFullName string, release model.ProjectRelease) ([]CodeSnippet, error) {
	// Implementation here would typically:
	// 1. Extract files from the release
	// 2. Parse files to identify meaningful code snippets
	// 3. Extract context information for each snippet

	// For now, return an empty slice as placeholder
	return []CodeSnippet{}, nil
}

// calculateSnippetHash generates a hash for a code snippet to identify duplicates
func calculateSnippetHash(snippet CodeSnippet) string {
	// Implementation could use a hash function like SHA-256
	// For now, return a simple placeholder
	return fmt.Sprintf("%s-%s", snippet.FilePath, snippet.Content)
}

// extractSPXAPIPatterns extracts API patterns from a project release
func (ctrl *Controller) extractSPXAPIPatterns(ctx context.Context, release model.ProjectRelease) (map[string]model.APIRecord, error) {
	// Implementation would analyze the release to identify API patterns
	// For now, return an empty map as placeholder
	return map[string]model.APIRecord{}, nil
}

// SaveKnowledgeBase persists the current state of the knowledge base to storage
func (ctrl *Controller) SaveKnowledgeBase(path string) error {
	// Implementation would save the knowledge base to a persistent storage
	// For example, save to disk or database
	return ctrl.knowledgeBase.Save(path)
}
