package model

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"github.com/DataIntelligenceCrew/go-faiss"
)

// 知识库核心结构定义
type KnowledgeBase struct {
	VectorDB     *faiss.IndexFlat     // 向量检索引擎
	CodeSnippets map[string]CodeMeta  // 代码片段元数据
	APIDocs      map[string]APIRecord // 提取的SPX API调用模式
	mu           sync.RWMutex         // Mutex for thread safety
}

// NewKnowledgeBase creates a new knowledge base instance
func NewKnowledgeBase(dimension int) (*KnowledgeBase, error) {
	// Initialize Faiss index for vector similarity search
	index, err := faiss.NewIndexFlatL2(dimension)
	if err != nil {
		return nil, fmt.Errorf("failed to create Faiss index: %w", err)
	}

	return &KnowledgeBase{
		VectorDB:     index,
		CodeSnippets: make(map[string]CodeMeta),
		APIDocs:      make(map[string]APIRecord),
		mu:           sync.RWMutex{},
	}, nil
}

type CodeMeta struct {
	ProjectID string
	FilePath  string
	CodeHash  string    // 用于增量更新
	Embedding []float32 // 向量
	Context   string    // 上下文描述（AI生成）
}

// APIRecord represents an SPX API usage pattern
type APIRecord struct {
	API      string
	Params   map[string]string
	Context  string
	Examples []string
}

// CodeSnippet represents a code snippet extracted from project files
type CodeSnippet struct {
	FilePath  string
	Content   string
	Context   string
	LineStart int
	LineEnd   int
}

// AddEmbedding adds an embedding vector to the Faiss index
func (kb *KnowledgeBase) AddEmbedding(embedding []float32) error {
	kb.mu.Lock()
	defer kb.mu.Unlock()

	// Add embedding to Faiss index
	err := kb.VectorDB.Add(embedding)
	return err
}

// AddCodeMeta adds code metadata to the knowledge base
func (kb *KnowledgeBase) AddCodeMeta(hash string, meta CodeMeta) {
	kb.mu.Lock()
	defer kb.mu.Unlock()
	kb.CodeSnippets[hash] = meta
}

// AddAPIRecord adds an API record to the knowledge base
func (kb *KnowledgeBase) AddAPIRecord(apiName string, record APIRecord) {
	kb.mu.Lock()
	defer kb.mu.Unlock()
	kb.APIDocs[apiName] = record
}

// Search searches for similar code snippets
func (kb *KnowledgeBase) Search(queryEmbedding []float32, k int64) ([]CodeMeta, error) {
	kb.mu.RLock()
	defer kb.mu.RUnlock()

	// Search in Faiss index
	ids, _, err := kb.VectorDB.Search(queryEmbedding, k)
	if err != nil {
		return nil, fmt.Errorf("failed to search in vector database: %w", err)
	}

	// Map IDs to code metadata
	results := make([]CodeMeta, 0, len(ids))
	idToMeta := make(map[float32]string)

	// Build reverse mapping from vector ID to code hash
	for hash, meta := range kb.CodeSnippets {
		// In a real implementation, you'd store the vector ID with the metadata
		// This is a simplified approach for the example
		for _, id := range ids {
			if calculateSimilarity(queryEmbedding, meta.Embedding) > 0.8 { // Threshold
				idToMeta[id] = hash
				break
			}
		}
	}

	// Collect results
	for _, id := range ids {
		if hash, ok := idToMeta[id]; ok {
			if meta, ok := kb.CodeSnippets[hash]; ok {
				results = append(results, meta)
			}
		}
	}

	return results, nil
}

// calculateSimilarity computes cosine similarity between vectors
func calculateSimilarity(a, b []float32) float32 {
	// Simplified similarity calculation
	// In production, use a proper vector math library
	return 0.9 // Placeholder
}

// Save writes the knowledge base state to disk
func (kb *KnowledgeBase) Save(path string) error {
	kb.mu.RLock()
	defer kb.mu.RUnlock()

	// Create directory if it doesn't exist
	if err := os.MkdirAll(path, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Save Faiss index
	indexPath := path + "/vectors.faiss"
	if err := faiss.WriteIndex(kb.VectorDB, indexPath); err != nil {
		return fmt.Errorf("failed to save vector index: %w", err)
	}

	// Save metadata
	metadataFile, err := os.Create(path + "/metadata.json")
	if err != nil {
		return fmt.Errorf("failed to create metadata file: %w", err)
	}
	defer metadataFile.Close()

	encoder := json.NewEncoder(metadataFile)
	if err := encoder.Encode(struct {
		CodeSnippets map[string]CodeMeta  `json:"codeSnippets"`
		APIDocs      map[string]APIRecord `json:"apiDocs"`
	}{
		CodeSnippets: kb.CodeSnippets,
		APIDocs:      kb.APIDocs,
	}); err != nil {
		return fmt.Errorf("failed to encode metadata: %w", err)
	}

	return nil
}

// Load reads the knowledge base state from disk
func (kb *KnowledgeBase) Load(path string) error {
	kb.mu.Lock()
	defer kb.mu.Unlock()

	// Load Faiss index
	indexPath := path + "/vectors.faiss"
	// Use the same dimension that was used to create the index
	// This value should match the dimension used in NewKnowledgeBase
	dimension := 128 // Adjust this value to match your vector dimension
	index, err := faiss.ReadIndex(indexPath, dimension)
	if err != nil {
		return fmt.Errorf("failed to load vector index: %w", err)
	}
	kb.VectorDB = index.AsFlat()

	// Load metadata
	metadataFile, err := os.Open(path + "/metadata.json")
	if err != nil {
		return fmt.Errorf("failed to open metadata file: %w", err)
	}
	defer metadataFile.Close()

	var metadata struct {
		CodeSnippets map[string]CodeMeta  `json:"codeSnippets"`
		APIDocs      map[string]APIRecord `json:"apiDocs"`
	}

	decoder := json.NewDecoder(metadataFile)
	if err := decoder.Decode(&metadata); err != nil {
		return fmt.Errorf("failed to decode metadata: %w", err)
	}

	kb.CodeSnippets = metadata.CodeSnippets
	kb.APIDocs = metadata.APIDocs

	return nil
}
