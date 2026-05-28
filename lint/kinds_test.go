package lint_test

import (
	"testing"

	"m31labs.dev/sirena"
	"m31labs.dev/sirena/lint"
)

// TestQueueWithoutPublisher fires SIR-KIND-QUEUE-NO-PUBLISHER when a queue
// has subscribers but no publisher. A queue with no producer is a stub or
// a typo — the v0.1 lint pack flags both the same way.
func TestQueueWithoutPublisher(t *testing.T) {
	root := tmpWorkspace(t, map[string]string{
		"x.sir": `queue events
service consumer
events -> consumer: subscribes`,
	})
	ws, err := sirena.OpenWorkspace(root)
	if err != nil {
		t.Fatal(err)
	}
	diags := lint.QueueWithoutPublisher(ws)
	var found bool
	for _, d := range diags {
		if d.Code == "SIR-KIND-QUEUE-NO-PUBLISHER" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected SIR-KIND-QUEUE-NO-PUBLISHER, got %+v", diags)
	}
}

// TestQueueWithoutSubscriber is the symmetric case: a queue with a
// publisher but no consumer. Almost always a stub or unfinished wire-up.
func TestQueueWithoutSubscriber(t *testing.T) {
	root := tmpWorkspace(t, map[string]string{
		"x.sir": `queue events
service producer
producer -> events: publishes`,
	})
	ws, err := sirena.OpenWorkspace(root)
	if err != nil {
		t.Fatal(err)
	}
	diags := lint.QueueWithoutSubscriber(ws)
	var found bool
	for _, d := range diags {
		if d.Code == "SIR-KIND-QUEUE-NO-SUBSCRIBER" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected SIR-KIND-QUEUE-NO-SUBSCRIBER, got %+v", diags)
	}
}

// TestQueueWellConnected_NoDiagnostic is the negative twin: a queue with
// both a publisher and a subscriber should not trip either rule.
func TestQueueWellConnected_NoDiagnostic(t *testing.T) {
	root := tmpWorkspace(t, map[string]string{
		"x.sir": `queue events
service producer
service consumer
producer -> events: publishes
events -> consumer: subscribes`,
	})
	ws, err := sirena.OpenWorkspace(root)
	if err != nil {
		t.Fatal(err)
	}
	var diags []sirena.Diagnostic
	diags = append(diags, lint.QueueWithoutPublisher(ws)...)
	diags = append(diags, lint.QueueWithoutSubscriber(ws)...)
	for _, d := range diags {
		if d.Code == "SIR-KIND-QUEUE-NO-PUBLISHER" || d.Code == "SIR-KIND-QUEUE-NO-SUBSCRIBER" {
			t.Errorf("well-connected queue should not flag: %+v", d)
		}
	}
}

// TestDeadElement flags an element with zero incoming AND zero outgoing
// edges. The lonely `service alone` here has no wires touching it; the
// other elements are connected and should NOT be flagged.
func TestDeadElement(t *testing.T) {
	root := tmpWorkspace(t, map[string]string{
		"x.sir": `service alone
service api
database db
api -> db: writes`,
	})
	ws, err := sirena.OpenWorkspace(root)
	if err != nil {
		t.Fatal(err)
	}
	diags := lint.DeadElement(ws)
	var foundAlone bool
	for _, d := range diags {
		if d.Code != "SIR-REF-DEAD-ELEMENT" {
			continue
		}
		// The two connected elements must NOT show up.
		if d.Message == "" {
			t.Errorf("dead-element diagnostic without message: %+v", d)
		}
		// Loose check: `alone` should be named in the message.
		if contains(d.Message, "alone") {
			foundAlone = true
		}
	}
	if !foundAlone {
		t.Errorf("expected SIR-REF-DEAD-ELEMENT for 'alone', got %+v", diags)
	}
	// And the connected pair must NOT be flagged.
	for _, d := range diags {
		if d.Code != "SIR-REF-DEAD-ELEMENT" {
			continue
		}
		if contains(d.Message, "\"api\"") || contains(d.Message, "\"db\"") {
			t.Errorf("connected element flagged as dead: %+v", d)
		}
	}
}

// contains is a tiny strings.Contains-equivalent kept local so the test
// file can avoid importing strings just for one helper.
func contains(s, sub string) bool {
	if len(sub) == 0 {
		return true
	}
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
