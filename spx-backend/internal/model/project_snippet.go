package model

// ProjectCodeSnippet represents a code snippet extracted from a project
type ProjectCodeSnippet struct {
	Model

	ProjectID int64  `gorm:"column:project_id;index"`
	ReleaseID int64  `gorm:"column:release_id;index"`
	FilePath  string `gorm:"column:file_path"`
	CodeHash  string `gorm:"column:code_hash"`
	VectorID  string `gorm:"column:vector_id"`
	Context   string `gorm:"column:context"`
}
