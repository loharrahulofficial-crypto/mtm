package tui

import (
	"fmt"
	"strings"
)

// Action names used for config overrides (keys: section of config.yaml) and
// help display.
const (
	aQuit         = "quit"
	aViewList     = "view_list"
	aViewBoard    = "view_board"
	aViewCalendar = "view_calendar"
	aCycleView    = "cycle_view"
	aUp           = "up"
	aDown         = "down"
	aLeft         = "left"
	aRight        = "right"
	aTop          = "top"
	aBottom       = "bottom"
	aPageDown     = "page_down"
	aPageUp       = "page_up"
	aSearch       = "search"
	aSearchNext   = "search_next"
	aSearchPrev   = "search_prev"
	aCommand      = "command"
	aAdd          = "add"
	aAddSubtask   = "subtask"
	aToggleDone   = "toggle_done"
	aDelete       = "delete"
	aStatus       = "status"
	aPriority     = "priority"
	aDue          = "due"
	aTags         = "tags"
	aTitle        = "title"
	aNotes        = "notes"
	aNotesEditor  = "notes_editor"
	aEditFile     = "edit_file"
	aArchive      = "archive"
	aSync         = "sync"
	aDetail       = "toggle_detail"
	aSidebar      = "toggle_sidebar"
	aHelp         = "help"
	aConflicts    = "conflicts"
	aFocusLeft    = "focus_left"
	aFocusRight   = "focus_right"
	aJumpLeft     = "jump_left"
	aJumpRight    = "jump_right"
)

var actionOrder = []string{
	aQuit, aViewList, aViewBoard, aViewCalendar, aCycleView,
	aUp, aDown, aLeft, aRight, aTop, aBottom, aPageDown, aPageUp,
	aSearch, aSearchNext, aSearchPrev, aCommand,
	aAdd, aAddSubtask, aToggleDone, aDelete, aStatus, aPriority, aDue, aTags, aTitle,
	aNotes, aNotesEditor, aEditFile, aArchive, aSync,
	aDetail, aSidebar, aFocusLeft, aFocusRight, aConflicts, aHelp,
	aJumpLeft, aJumpRight,
}

var actionDescriptions = map[string]string{
	aQuit:         "quit",
	aViewList:     "list view",
	aViewBoard:    "board view",
	aViewCalendar: "calendar view",
	aCycleView:    "cycle views",
	aUp:           "move up",
	aDown:         "move down",
	aLeft:         "move left / column",
	aRight:        "move right / column",
	aTop:          "jump to first",
	aBottom:       "jump to last",
	aPageDown:     "page down",
	aPageUp:       "page up",
	aSearch:       "search / filter",
	aSearchNext:   "next match",
	aSearchPrev:   "previous match",
	aCommand:      "command line",
	aAdd:          "add task (quick-add)",
	aAddSubtask:   "add subtask of selected",
	aToggleDone:   "mark done / un-done",
	aDelete:       "delete (asks first)",
	aStatus:       "change status",
	aPriority:     "change priority",
	aDue:          "change due date",
	aTags:         "change tags",
	aTitle:        "edit title",
	aNotes:        "notes (built-in editor)",
	aNotesEditor:  "notes in $EDITOR",
	aEditFile:     "edit task file in $EDITOR",
	aArchive:      "archive done tasks / selected task",
	aSync:         "sync now",
	aDetail:       "toggle detail pane",
	aSidebar:      "toggle sidebar",
	aFocusLeft:    "focus pane left",
	aFocusRight:   "focus pane right",
	aConflicts:    "open conflict screen",
	aHelp:         "toggle this help",
	aJumpLeft:     "board: move card left · calendar: prev month",
	aJumpRight:    "board: move card right · calendar: next month",
}

// KeyMap maps action -> key sequence (e.g. "gg", "ctrl+d", "?"). Built from
// the defaults, optionally overridden via config.yaml keys: (action name ->
// key). Matching happens on normalized key strings.
type KeyMap struct {
	binds map[string]string
}

// DefaultKeyMap returns the built-in bindings.
func DefaultKeyMap() KeyMap {
	return KeyMap{binds: map[string]string{
		aQuit:         "q",
		aViewList:     "1",
		aViewBoard:    "2",
		aViewCalendar: "3",
		aCycleView:    "tab",
		aUp:           "k",
		aDown:         "j",
		aLeft:         "h",
		aRight:        "l",
		aTop:          "gg",
		aBottom:       "G",
		aPageDown:     "ctrl+d",
		aPageUp:       "ctrl+u",
		aSearch:       "/",
		aSearchNext:   "n",
		aSearchPrev:   "N",
		aCommand:      ":",
		aAdd:          "a",
		aAddSubtask:   "A",
		aToggleDone:   "x",
		aDelete:       "dd",
		aStatus:       "s",
		aPriority:     "p",
		aDue:          "D",
		aTags:         "t",
		aTitle:        "e",
		aNotes:        "m",
		aNotesEditor:  "M",
		aEditFile:     "E",
		aArchive:      "X",
		aSync:         "S",
		aDetail:       "v",
		aSidebar:      "ctrl+b",
		aFocusLeft:    "ctrl+h",
		aFocusRight:   "ctrl+l",
		aConflicts:    "ctrl+x",
		aHelp:         "?",
		aJumpLeft:     "H",
		aJumpRight:    "L",
	}}
}

// Seq returns the key sequence bound to action.
func (k KeyMap) Seq(action string) string { return k.binds[action] }

// Binds returns a copy of the action -> sequence map.
func (k KeyMap) Binds() map[string]string {
	out := make(map[string]string, len(k.binds))
	for a, s := range k.binds {
		out[a] = s
	}
	return out
}

// ApplyOverrides applies config keys: (action -> sequence). Unknown actions
// are skipped and reported (never fatal).
func (k KeyMap) ApplyOverrides(overrides map[string]string) []string {
	var unknown []string
	for action, seq := range overrides {
		if _, ok := k.binds[action]; !ok {
			unknown = append(unknown, action)
			continue
		}
		seq = strings.TrimSpace(seq)
		if seq == "" {
			continue
		}
		k.binds[action] = seq
	}
	return unknown
}

// IsPrefix reports whether s is a strict prefix of some binding (used to
// hold multi-key sequences like "gg" and "dd").
func (k KeyMap) IsPrefix(s string) bool {
	for _, seq := range k.binds {
		if strings.HasPrefix(seq, s) && len(seq) > len(s) {
			return true
		}
	}
	return false
}

// Match returns the action bound exactly to seq, or "" if none.
func (k KeyMap) Match(seq string) string {
	for action, bound := range k.binds {
		if bound == seq {
			return action
		}
	}
	return ""
}

// HelpEntries returns {action, description, sequence} in stable display order.
func HelpEntries(k KeyMap) [][3]string {
	var out [][3]string
	for _, action := range actionOrder {
		seq := k.binds[action]
		if seq == "" {
			continue
		}
		desc := actionDescriptions[action]
		if desc == "" {
			desc = action
		}
		out = append(out, [3]string{action, desc, seq})
	}
	return out
}

// UnknownActionsNote describes unrecognized config keys, or "" if none.
func UnknownActionsNote(unknown []string) string {
	if len(unknown) == 0 {
		return ""
	}
	return fmt.Sprintf("ignored unknown keys: entries: %s", strings.Join(unknown, ", "))
}
