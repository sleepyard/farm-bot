package gamestate

import (
	"encoding/json"
	"fmt"
	"strings"
)

// 宽松外壳：消息体逐条 RawMessage 解析，避免单条字段类型异常拖垮整包。
type looseEnvelope struct {
	GreToClientEvent *struct {
		GreToClientMessages []json.RawMessage `json:"greToClientMessages"`
	} `json:"greToClientEvent"`
}

// ParseLogLine 从一行 Player.log 提取 greToClientEvent（跳过 Unity 前缀）。
// 非 GRE 行返回 (nil, false)。
func ParseLogLine(line string) (*greEnvelope, bool) {
	if !strings.Contains(line, "greToClientEvent") {
		return nil, false
	}
	i := strings.Index(line, "{")
	if i < 0 {
		return nil, false
	}
	raw := []byte(line[i:])

	var loose looseEnvelope
	if err := json.Unmarshal(raw, &loose); err != nil || loose.GreToClientEvent == nil {
		return nil, false
	}

	msgs := make([]greMessage, 0, len(loose.GreToClientEvent.GreToClientMessages))
	for _, rm := range loose.GreToClientEvent.GreToClientMessages {
		var msg greMessage
		if err := json.Unmarshal(rm, &msg); err != nil {
			continue // 跳过坏消息，保留同包其它 GSM/AAR
		}
		if msg.GameStateMessage != nil {
			for i := range msg.GameStateMessage.GameObjects {
				msg.GameStateMessage.GameObjects[i].normalizeStats()
			}
		}
		msgs = append(msgs, msg)
	}
	if len(msgs) == 0 {
		return nil, false
	}
	return &greEnvelope{
		GreToClientEvent: &greToClientEvent{GreToClientMessages: msgs},
	}, true
}

// IngestChunk 把一段日志增量按行解析并合并进 Store；返回是否有更新。
func IngestChunk(store *Store, chunk string) bool {
	if store == nil || chunk == "" {
		return false
	}
	changed := false
	for _, line := range strings.Split(chunk, "\n") {
		env, ok := ParseLogLine(line)
		if !ok {
			continue
		}
		if store.ApplyEnvelope(env) {
			changed = true
		}
	}
	return changed
}

func formatSummary(matchID string, seat int, turn TurnInfo, ours bool, plays, casts, objs, pending int) string {
	mid := matchID
	if len(mid) > 8 {
		mid = mid[:8]
	}
	return fmt.Sprintf(
		"seat=%d ours=%v turn=%d %s/%s play=%d cast=%d objs=%d pending=%d match=%s",
		seat, ours, turn.TurnNumber, turn.Phase, turn.Step, plays, casts, objs, pending, mid,
	)
}
