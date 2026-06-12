import React from "react";
import { CheckCircle2, ChevronDown, ChevronRight, Circle } from "lucide-react";
import type { LessonWithStatus } from "../../lib/api/types";

export function LessonsPanel({ lessons, isBusy }: { lessons: LessonWithStatus[]; isBusy: boolean }) {
  const [openLessonId, setOpenLessonId] = React.useState<string | null>(null);

  if (lessons.length === 0) {
    return <div className="empty">{isBusy ? "教材を読み込んでいます" : "教材を読み込めませんでした"}</div>;
  }

  const completedCount = lessons.filter((lesson) => lesson.completed).length;

  return (
    <div className="lessons">
      <div className="lessons-progress">
        <div className="lessons-progress-bar">
          <div
            className="lessons-progress-fill"
            style={{ width: `${(completedCount / lessons.length) * 100}%` }}
          />
        </div>
        <span>
          {completedCount}/{lessons.length} 完了
        </span>
      </div>
      <div className="lessons-list">
        {lessons.map((lesson) => {
          const isOpen = openLessonId === lesson.id;
          const StatusIcon = lesson.completed ? CheckCircle2 : Circle;
          const ToggleIcon = isOpen ? ChevronDown : ChevronRight;
          return (
            <div className={`lesson-item ${lesson.completed ? "completed" : ""}`} key={lesson.id}>
              <button
                className="lesson-header"
                onClick={() => setOpenLessonId(isOpen ? null : lesson.id)}
                aria-expanded={isOpen}
              >
                <StatusIcon size={18} className="lesson-status-icon" />
                <span className="lesson-order">Lesson {lesson.order}</span>
                <strong>{lesson.title}</strong>
                <ToggleIcon size={16} className="lesson-toggle-icon" />
              </button>
              {isOpen && (
                <div className="lesson-body">
                  <p>{lesson.description}</p>
                  <div className="lesson-goal">
                    <span>達成条件</span>
                    <strong>{lesson.goal}</strong>
                  </div>
                  <ol>
                    {lesson.steps.map((step) => (
                      <li key={step}>{step}</li>
                    ))}
                  </ol>
                  {lesson.completed && lesson.completed_at && (
                    <p className="lesson-completed-at">
                      達成: {new Date(lesson.completed_at).toLocaleString()}
                    </p>
                  )}
                </div>
              )}
            </div>
          );
        })}
      </div>
    </div>
  );
}
