package copilot

import (
	"context"
	"hash/fnv"
	"math"
	"strings"
	"unicode"
)

// EmbeddingService 定义了文本向量化服务的接口
type EmbeddingService interface {
	GetEmbedding(ctx context.Context, text string) ([]float32, error)
}

// 基于哈希技巧的本地文本向量化方案
type LocalEmbedder struct {
	dim        int                 // 向量维度 (384)
	stopWords  map[string]struct{} // 停用词表
	wordWeight map[string]float32  // 词语权重
}

func NewLocalEmbedder() *LocalEmbedder {
	return &LocalEmbedder{
		dim: 384,
		stopWords: map[string]struct{}{
			"的": {}, "是": {}, "在": {}, "了": {}, "和": {},
			// 可扩展更多中文停用词...
		},
		wordWeight: make(map[string]float32),
	}
}

// 实现文本到384维向量的本地转换
func (e *LocalEmbedder) GetEmbedding(ctx context.Context, text string) ([]float32, error) {
	// 文本预处理
	words := e.tokenize(text)

	// 初始化向量
	vector := make([]float32, e.dim)

	// 分布哈希策略
	for _, word := range words {
		if e.isStopWord(word) {
			continue
		}

		// 获取词语权重
		weight := e.getWordWeight(word)

		// 使用双重哈希生成分布
		h1, h2 := e.doubleHash(word)
		idx1 := h1 % uint32(e.dim)
		idx2 := h2 % uint32(e.dim)

		vector[idx1] += weight
		vector[idx2] += weight * 0.5
	}

	// 归一化处理
	e.normalize(vector)
	return vector, nil
}

// 文本分词（简单按unicode分隔）
func (e *LocalEmbedder) tokenize(text string) []string {
	var words []string
	runes := []rune(text)

	for i := 0; i < len(runes); {
		// 跳过标点符号
		for i < len(runes) && !unicode.IsLetter(runes[i]) {
			i++
		}

		// 捕获词语
		start := i
		for i < len(runes) && (unicode.IsLetter(runes[i]) || unicode.IsNumber(runes[i])) {
			i++
		}
		if start < i {
			words = append(words, strings.ToLower(string(runes[start:i])))
		}
	}
	return words
}

// 双重哈希函数生成
func (e *LocalEmbedder) doubleHash(word string) (uint32, uint32) {
	h1 := fnv.New32a()
	h1.Write([]byte(word))

	h2 := fnv.New32a()
	h2.Write([]byte(reverseString(word)))

	return h1.Sum32(), h2.Sum32()
}

// 字符串反转辅助函数
func reverseString(s string) string {
	runes := []rune(s)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}

// 停用词过滤
func (e *LocalEmbedder) isStopWord(word string) bool {
	_, exists := e.stopWords[word]
	return exists
}

// 基于伪词频的权重计算
func (e *LocalEmbedder) getWordWeight(word string) float32 {
	if w, exists := e.wordWeight[word]; exists {
		return w
	}

	// 模拟词频统计（实际应用需要预训练数据）
	h := fnv.New32a()
	h.Write([]byte(word))
	weight := 1.0 / (1.0 + math.Exp(-float64(h.Sum32()%100)/10))

	e.wordWeight[word] = float32(weight)
	return e.wordWeight[word]
}

// L2归一化
func (e *LocalEmbedder) normalize(vec []float32) {
	var sum float32
	for _, v := range vec {
		sum += v * v
	}

	if sum == 0 {
		return
	}

	norm := float32(math.Sqrt(float64(sum)))
	for i := range vec {
		vec[i] /= norm
	}
}
