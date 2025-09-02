package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"claude-code-intelligence/internal/database"
	"claude-code-intelligence/internal/types"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

// MemorySystem handles project memory consolidation and pattern recognition
type MemorySystem struct {
	db       *database.Manager
	ollama   *OllamaClient
	logger   *logrus.Logger
}

// NewMemorySystem creates a new memory system
func NewMemorySystem(db *database.Manager, ollama *OllamaClient, logger *logrus.Logger) *MemorySystem {
	return &MemorySystem{
		db:     db,
		ollama: ollama,
		logger: logger,
	}
}

// ProjectMemory represents consolidated project knowledge
type ProjectMemory struct {
	ProjectID        string                `json:"project_id"`
	ConsolidatedAt   time.Time             `json:"consolidated_at"`
	SessionCount     int                   `json:"session_count"`
	Topics           []ConsolidatedTopic   `json:"topics"`
	Decisions        []ConsolidatedDecision `json:"decisions"`
	Patterns         []Pattern             `json:"patterns"`
	Timeline         []TimelineEvent       `json:"timeline"`
	KeyInsights      []string              `json:"key_insights"`
	TechnicalStack   []string              `json:"technical_stack"`
	CommonIssues     []Issue               `json:"common_issues"`
}

// ConsolidatedTopic represents a topic across multiple sessions
type ConsolidatedTopic struct {
	Topic       string    `json:"topic"`
	Frequency   int       `json:"frequency"`
	Importance  float64   `json:"importance"`
	FirstSeen   time.Time `json:"first_seen"`
	LastSeen    time.Time `json:"last_seen"`
	Evolution   []string  `json:"evolution"` // How the topic evolved over time
	RelatedTopics []string `json:"related_topics"`
}

// ConsolidatedDecision represents a decision with full context
type ConsolidatedDecision struct {
	Decision     string    `json:"decision"`
	Reasoning    string    `json:"reasoning"`
	Outcome      string    `json:"outcome"`
	Impact       string    `json:"impact"`
	MadeAt       time.Time `json:"made_at"`
	SessionID    string    `json:"session_id"`
	Alternatives []string  `json:"alternatives"`
	LessonsLearned string  `json:"lessons_learned"`
}

// Pattern represents a recurring pattern in the project
type Pattern struct {
	Type        string   `json:"type"` // error_pattern, solution_pattern, workflow_pattern
	Description string   `json:"description"`
	Occurrences int      `json:"occurrences"`
	Examples    []string `json:"examples"`
	Recommendation string `json:"recommendation"`
}

// TimelineEvent represents a significant event in project history
type TimelineEvent struct {
	Timestamp   time.Time `json:"timestamp"`
	Type        string    `json:"type"` // milestone, issue, decision, breakthrough
	Description string    `json:"description"`
	SessionID   string    `json:"session_id"`
	Impact      string    `json:"impact"`
}

// Issue represents a common problem and its solutions
type Issue struct {
	Problem   string   `json:"problem"`
	Solutions []string `json:"solutions"`
	Frequency int      `json:"frequency"`
	Resolved  bool     `json:"resolved"`
}

// ConsolidateProjectMemory consolidates knowledge from all project sessions
func (ms *MemorySystem) ConsolidateProjectMemory(ctx context.Context, projectID string) (*ProjectMemory, error) {
	startTime := time.Now()
	
	ms.logger.WithField("project_id", projectID).Info("Starting project memory consolidation")

	// Get all sessions for the project
	sessions, err := ms.db.ListSessions(ctx, 1000, 0, &projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to list project sessions: %w", err)
	}

	if len(sessions) == 0 {
		return &ProjectMemory{
			ProjectID:      projectID,
			ConsolidatedAt: time.Now(),
			SessionCount:   0,
		}, nil
	}

	// Collect all topics and decisions
	allTopics := make([]types.Topic, 0)
	allDecisions := make([]types.Decision, 0)
	
	for _, session := range sessions {
		topics, err := ms.db.GetSessionTopics(ctx, session.ID)
		if err == nil {
			allTopics = append(allTopics, topics...)
		}
		
		decisions, err := ms.db.GetSessionDecisions(ctx, session.ID)
		if err == nil {
			allDecisions = append(allDecisions, decisions...)
		}
	}

	// Consolidate topics
	consolidatedTopics := ms.consolidateTopics(allTopics)

	// Consolidate decisions
	consolidatedDecisions := ms.consolidateDecisions(allDecisions, sessions)

	// Identify patterns
	patterns := ms.identifyPatterns(sessions, allTopics, allDecisions)

	// Build timeline
	timeline := ms.buildTimeline(sessions, allDecisions)

	// Extract key insights using AI
	keyInsights := ms.extractKeyInsights(ctx, sessions, consolidatedTopics, consolidatedDecisions)

	// Identify technical stack
	technicalStack := ms.identifyTechnicalStack(allTopics, sessions)

	// Identify common issues
	commonIssues := ms.identifyCommonIssues(sessions, allTopics)

	memory := &ProjectMemory{
		ProjectID:        projectID,
		ConsolidatedAt:   time.Now(),
		SessionCount:     len(sessions),
		Topics:           consolidatedTopics,
		Decisions:        consolidatedDecisions,
		Patterns:         patterns,
		Timeline:         timeline,
		KeyInsights:      keyInsights,
		TechnicalStack:   technicalStack,
		CommonIssues:     commonIssues,
	}

	// Store consolidated memory
	if err := ms.storeProjectMemory(ctx, memory); err != nil {
		ms.logger.WithError(err).Warn("Failed to store project memory")
	}

	ms.logger.WithFields(logrus.Fields{
		"project_id":     projectID,
		"sessions":       len(sessions),
		"topics":         len(consolidatedTopics),
		"decisions":      len(consolidatedDecisions),
		"patterns":       len(patterns),
		"insights":       len(keyInsights),
		"processing_time": time.Since(startTime),
	}).Info("Project memory consolidation completed")

	return memory, nil
}

// consolidateTopics merges and analyzes topics across sessions
func (ms *MemorySystem) consolidateTopics(topics []types.Topic) []ConsolidatedTopic {
	topicMap := make(map[string]*ConsolidatedTopic)
	
	for _, topic := range topics {
		key := strings.ToLower(topic.Topic)
		
		if existing, exists := topicMap[key]; exists {
			existing.Frequency++
			existing.Importance = (existing.Importance + topic.RelevanceScore) / 2
			
			if topic.FirstMentionedAt != nil && topic.FirstMentionedAt.Before(existing.FirstSeen) {
				existing.FirstSeen = *topic.FirstMentionedAt
			}
			if topic.FirstMentionedAt != nil && topic.FirstMentionedAt.After(existing.LastSeen) {
				existing.LastSeen = *topic.FirstMentionedAt
			}
		} else {
			firstSeen := time.Now()
			if topic.FirstMentionedAt != nil {
				firstSeen = *topic.FirstMentionedAt
			}
			
			topicMap[key] = &ConsolidatedTopic{
				Topic:      topic.Topic,
				Frequency:  1,
				Importance: topic.RelevanceScore,
				FirstSeen:  firstSeen,
				LastSeen:   firstSeen,
				Evolution:  []string{},
			}
		}
	}

	// Find related topics through co-occurrence
	for _, topic1 := range topicMap {
		related := make([]string, 0)
		for _, topic2 := range topicMap {
			if topic1.Topic != topic2.Topic && ms.areTopicsRelated(topic1.Topic, topic2.Topic) {
				related = append(related, topic2.Topic)
			}
		}
		topic1.RelatedTopics = related
	}

	// Convert map to slice and sort by importance
	consolidated := make([]ConsolidatedTopic, 0, len(topicMap))
	for _, topic := range topicMap {
		consolidated = append(consolidated, *topic)
	}
	
	sort.Slice(consolidated, func(i, j int) bool {
		return consolidated[i].Importance > consolidated[j].Importance
	})

	return consolidated
}

// consolidateDecisions analyzes and groups decisions
func (ms *MemorySystem) consolidateDecisions(decisions []types.Decision, sessions []*types.Session) []ConsolidatedDecision {
	consolidated := make([]ConsolidatedDecision, 0, len(decisions))
	
	// Create session map for quick lookup
	sessionMap := make(map[string]*types.Session)
	for _, session := range sessions {
		sessionMap[session.ID] = session
	}
	
	for _, decision := range decisions {
		cd := ConsolidatedDecision{
			Decision:  decision.DecisionText,
			MadeAt:    decision.CreatedAt,
			SessionID: decision.SessionID,
		}
		
		if decision.Reasoning != nil {
			cd.Reasoning = *decision.Reasoning
		}
		if decision.Outcome != nil {
			cd.Outcome = *decision.Outcome
		}
		
		// Extract alternatives and lessons from tags if available
		if decision.Tags != "" {
			var tags []string
			if err := json.Unmarshal([]byte(decision.Tags), &tags); err == nil {
				cd.Alternatives = tags
			}
		}
		
		consolidated = append(consolidated, cd)
	}
	
	// Sort by timestamp
	sort.Slice(consolidated, func(i, j int) bool {
		return consolidated[i].MadeAt.After(consolidated[j].MadeAt)
	})
	
	return consolidated
}

// identifyPatterns finds recurring patterns in the project
func (ms *MemorySystem) identifyPatterns(sessions []*types.Session, topics []types.Topic, decisions []types.Decision) []Pattern {
	patterns := make([]Pattern, 0)
	
	// Pattern 1: Error patterns (topics containing "error", "bug", "issue")
	errorPattern := ms.findErrorPatterns(topics, sessions)
	if errorPattern != nil {
		patterns = append(patterns, *errorPattern)
	}
	
	// Pattern 2: Solution patterns (decisions that resolved issues)
	solutionPattern := ms.findSolutionPatterns(decisions)
	if solutionPattern != nil {
		patterns = append(patterns, *solutionPattern)
	}
	
	// Pattern 3: Workflow patterns (recurring sequences of topics)
	workflowPattern := ms.findWorkflowPatterns(sessions, topics)
	if workflowPattern != nil {
		patterns = append(patterns, *workflowPattern)
	}
	
	return patterns
}

// findErrorPatterns identifies common error patterns
func (ms *MemorySystem) findErrorPatterns(topics []types.Topic, sessions []*types.Session) *Pattern {
	errorKeywords := []string{"error", "bug", "issue", "problem", "fail", "crash"}
	errorTopics := make([]string, 0)
	
	for _, topic := range topics {
		topicLower := strings.ToLower(topic.Topic)
		for _, keyword := range errorKeywords {
			if strings.Contains(topicLower, keyword) {
				errorTopics = append(errorTopics, topic.Topic)
				break
			}
		}
	}
	
	if len(errorTopics) > 0 {
		return &Pattern{
			Type:        "error_pattern",
			Description: "Common errors and issues encountered",
			Occurrences: len(errorTopics),
			Examples:    errorTopics[:min(5, len(errorTopics))],
			Recommendation: "Consider implementing better error handling and prevention strategies",
		}
	}
	
	return nil
}

// findSolutionPatterns identifies successful problem-solving patterns
func (ms *MemorySystem) findSolutionPatterns(decisions []types.Decision) *Pattern {
	solutionKeywords := []string{"fixed", "resolved", "solved", "implemented", "improved"}
	solutions := make([]string, 0)
	
	for _, decision := range decisions {
		decisionLower := strings.ToLower(decision.DecisionText)
		for _, keyword := range solutionKeywords {
			if strings.Contains(decisionLower, keyword) {
				solutions = append(solutions, decision.DecisionText)
				break
			}
		}
	}
	
	if len(solutions) > 0 {
		return &Pattern{
			Type:        "solution_pattern",
			Description: "Successful problem-solving approaches",
			Occurrences: len(solutions),
			Examples:    solutions[:min(5, len(solutions))],
			Recommendation: "These approaches have proven effective in the past",
		}
	}
	
	return nil
}

// findWorkflowPatterns identifies recurring workflow patterns
func (ms *MemorySystem) findWorkflowPatterns(sessions []*types.Session, topics []types.Topic) *Pattern {
	// Group topics by session
	sessionTopics := make(map[string][]string)
	for _, topic := range topics {
		sessionTopics[topic.SessionID] = append(sessionTopics[topic.SessionID], topic.Topic)
	}
	
	// Look for common sequences
	sequences := make(map[string]int)
	for _, topics := range sessionTopics {
		if len(topics) >= 2 {
			for i := 0; i < len(topics)-1; i++ {
				sequence := fmt.Sprintf("%s -> %s", topics[i], topics[i+1])
				sequences[sequence]++
			}
		}
	}
	
	// Find most common sequences
	var commonSequences []string
	for seq, count := range sequences {
		if count >= 2 {
			commonSequences = append(commonSequences, seq)
		}
	}
	
	if len(commonSequences) > 0 {
		return &Pattern{
			Type:        "workflow_pattern",
			Description: "Common workflow sequences",
			Occurrences: len(commonSequences),
			Examples:    commonSequences[:min(5, len(commonSequences))],
			Recommendation: "These workflows appear frequently and might benefit from automation",
		}
	}
	
	return nil
}

// buildTimeline creates a chronological timeline of significant events
func (ms *MemorySystem) buildTimeline(sessions []*types.Session, decisions []types.Decision) []TimelineEvent {
	events := make([]TimelineEvent, 0)
	
	// Add session creation as milestones
	for _, session := range sessions {
		if session.CompressionRatio > 0.5 { // Significant sessions
			events = append(events, TimelineEvent{
				Timestamp:   session.CreatedAt,
				Type:        "milestone",
				Description: fmt.Sprintf("Session: %s", session.Name),
				SessionID:   session.ID,
				Impact:      "Session created",
			})
		}
	}
	
	// Add decisions as events
	for _, decision := range decisions {
		if decision.ImportanceScore > 0.7 { // Important decisions
			events = append(events, TimelineEvent{
				Timestamp:   decision.CreatedAt,
				Type:        "decision",
				Description: decision.DecisionText,
				SessionID:   decision.SessionID,
				Impact:      "High importance decision",
			})
		}
	}
	
	// Sort by timestamp
	sort.Slice(events, func(i, j int) bool {
		return events[i].Timestamp.Before(events[j].Timestamp)
	})
	
	return events
}

// extractKeyInsights uses AI to extract key insights
func (ms *MemorySystem) extractKeyInsights(ctx context.Context, sessions []*types.Session, topics []ConsolidatedTopic, decisions []ConsolidatedDecision) []string {
	// Build a summary of the project for AI analysis
	var summaryParts []string
	
	summaryParts = append(summaryParts, fmt.Sprintf("Project has %d sessions", len(sessions)))
	
	if len(topics) > 0 {
		summaryParts = append(summaryParts, "\nTop topics:")
		for i, topic := range topics[:min(10, len(topics))] {
			summaryParts = append(summaryParts, fmt.Sprintf("%d. %s (frequency: %d)", i+1, topic.Topic, topic.Frequency))
		}
	}
	
	if len(decisions) > 0 {
		summaryParts = append(summaryParts, "\nKey decisions:")
		for i, decision := range decisions[:min(5, len(decisions))] {
			summaryParts = append(summaryParts, fmt.Sprintf("%d. %s", i+1, decision.Decision))
		}
	}
	
	// For now, return basic insights
	// In production, this would call the Ollama API for deeper analysis
	insights := []string{
		fmt.Sprintf("Project contains %d sessions with %d key topics", len(sessions), len(topics)),
		fmt.Sprintf("%d important decisions have been made", len(decisions)),
	}
	
	if len(topics) > 0 {
		insights = append(insights, fmt.Sprintf("Most discussed topic: %s", topics[0].Topic))
	}
	
	return insights
}

// identifyTechnicalStack identifies technologies used in the project
func (ms *MemorySystem) identifyTechnicalStack(topics []types.Topic, sessions []*types.Session) []string {
	techKeywords := map[string]bool{
		"go": true, "golang": true, "python": true, "javascript": true, "typescript": true,
		"react": true, "vue": true, "angular": true, "node": true, "express": true,
		"django": true, "flask": true, "gin": true, "echo": true,
		"postgresql": true, "mysql": true, "mongodb": true, "redis": true, "sqlite": true,
		"docker": true, "kubernetes": true, "aws": true, "gcp": true, "azure": true,
		"git": true, "github": true, "gitlab": true,
	}
	
	stack := make(map[string]bool)
	
	// Check topics
	for _, topic := range topics {
		words := strings.Fields(strings.ToLower(topic.Topic))
		for _, word := range words {
			if techKeywords[word] {
				stack[word] = true
			}
		}
	}
	
	// Check session summaries
	for _, session := range sessions {
		if session.Summary != nil {
			words := strings.Fields(strings.ToLower(*session.Summary))
			for _, word := range words {
				if techKeywords[word] {
					stack[word] = true
				}
			}
		}
	}
	
	// Convert to slice
	result := make([]string, 0, len(stack))
	for tech := range stack {
		result = append(result, tech)
	}
	
	sort.Strings(result)
	return result
}

// identifyCommonIssues finds recurring problems
func (ms *MemorySystem) identifyCommonIssues(sessions []*types.Session, topics []types.Topic) []Issue {
	issueMap := make(map[string]*Issue)
	
	problemKeywords := []string{"error", "bug", "issue", "problem", "fail"}
	solutionKeywords := []string{"fix", "solve", "resolve", "workaround"}
	
	for _, topic := range topics {
		topicLower := strings.ToLower(topic.Topic)
		
		// Check if it's a problem
		isProblem := false
		for _, keyword := range problemKeywords {
			if strings.Contains(topicLower, keyword) {
				isProblem = true
				break
			}
		}
		
		if isProblem {
			if issue, exists := issueMap[topic.Topic]; exists {
				issue.Frequency++
			} else {
				issueMap[topic.Topic] = &Issue{
					Problem:   topic.Topic,
					Solutions: []string{},
					Frequency: 1,
					Resolved:  false,
				}
			}
			
			// Check for solutions in the same context
			if topic.Context != nil {
				contextLower := strings.ToLower(*topic.Context)
				for _, keyword := range solutionKeywords {
					if strings.Contains(contextLower, keyword) {
						issueMap[topic.Topic].Resolved = true
						issueMap[topic.Topic].Solutions = append(issueMap[topic.Topic].Solutions, *topic.Context)
						break
					}
				}
			}
		}
	}
	
	// Convert to slice
	issues := make([]Issue, 0, len(issueMap))
	for _, issue := range issueMap {
		issues = append(issues, *issue)
	}
	
	// Sort by frequency
	sort.Slice(issues, func(i, j int) bool {
		return issues[i].Frequency > issues[j].Frequency
	})
	
	return issues
}

// areTopicsRelated checks if two topics are related
func (ms *MemorySystem) areTopicsRelated(topic1, topic2 string) bool {
	// Simple relatedness check based on common words
	words1 := strings.Fields(strings.ToLower(topic1))
	words2 := strings.Fields(strings.ToLower(topic2))
	
	commonWords := 0
	for _, w1 := range words1 {
		for _, w2 := range words2 {
			if w1 == w2 && len(w1) > 3 { // Ignore short words
				commonWords++
			}
		}
	}
	
	return commonWords >= 2
}

// storeProjectMemory stores the consolidated memory in the database
func (ms *MemorySystem) storeProjectMemory(ctx context.Context, memory *ProjectMemory) error {
	// Serialize memory to JSON
	memoryJSON, err := json.Marshal(memory)
	if err != nil {
		return fmt.Errorf("failed to serialize project memory: %w", err)
	}
	
	// Update or create project record with consolidated memory
	query := `
		UPDATE projects 
		SET metadata = ?, last_active = CURRENT_TIMESTAMP 
		WHERE id = ?
	`
	
	_, err = ms.db.ExecContext(ctx, query, string(memoryJSON), memory.ProjectID)
	if err != nil {
		// If project doesn't exist, create it
		insertQuery := `
			INSERT INTO projects (id, name, path, metadata, created_at, last_active)
			VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		`
		projectID := memory.ProjectID
		if projectID == "" {
			projectID = uuid.New().String()
		}
		_, err = ms.db.ExecContext(ctx, insertQuery, projectID, "Project", ".", string(memoryJSON))
	}
	
	return err
}

// GetProjectMemory retrieves consolidated project memory
func (ms *MemorySystem) GetProjectMemory(ctx context.Context, projectID string) (*ProjectMemory, error) {
	query := `SELECT metadata FROM projects WHERE id = ?`
	
	var metadataJSON string
	err := ms.db.QueryRowContext(ctx, query, projectID).Scan(&metadataJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to get project memory: %w", err)
	}
	
	var memory ProjectMemory
	if err := json.Unmarshal([]byte(metadataJSON), &memory); err != nil {
		return nil, fmt.Errorf("failed to parse project memory: %w", err)
	}
	
	return &memory, nil
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}