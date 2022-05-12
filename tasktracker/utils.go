package main

import (
	"github.com/google/uuid"
)

func fromTaskToEvent(task Task, uuid uuid.UUID) TaskEvent {
	return TaskEvent{
		Description: task.Description,
		IsOpen:      task.IsOpen,
		PopugID:     task.PopugID,
		PublicID:    uuid,
	}
}
