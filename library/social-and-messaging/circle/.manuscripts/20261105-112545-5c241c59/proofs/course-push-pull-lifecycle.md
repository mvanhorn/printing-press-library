# course push/pull live lifecycle (2026-11-05), course space 1001068 (pp-test-course)
## push preview
{
  "applied": false,
  "space_id": 1001068,
  "plan": [
    {
      "action": "create",
      "kind": "section",
      "path": "01-pp-test-module-one",
      "name": "pp-test Module One",
      "id": -1
    },
    {
      "action": "create",
      "kind": "lesson",
      "path": "01-pp-test-module-one/01-lesson-one.md",
      "name": "pp-test Lesson One",
      "id": -2
    },
    {
      "action": "upload",
      "kind": "media",
      "path": "01-pp-test-module-one/01-lesson-one.md",
      "name": "pp-test Lesson One",
      "id": -2
    },
    {
      "action": "create",
      "kind": "lesson",
      "path": "01-pp-test-module-one/02-lesson-two.md",
      "name": "pp-test Lesson Two",
      "id": -3
    },
    {
      "action": "create",
      "kind": "section",
      "path": "02-pp-test-module-two",
      "name": "Pp Test Module Two",
      "id": -4
    },
    {
      "action": "create",
      "kind": "lesson",
      "path": "02-pp-test-module-two/01-lesson-three.md",
      "name": "pp-test Lesson Three",
      "id": -5
    },
    {
      "action": "reorder",
      "kind": "course",
      "path": "",
      "name": "",
      "id": 1001068
    }
  ],
  "summary": {
    "create": 5,
    "reorder": 1,
    "upload": 1
  }
}
## push --yes
{
  "applied": true,
  "space_id": 1001068,
  "plan": [
    {
      "action": "create",
      "kind": "section",
      "path": "01-pp-test-module-one",
      "name": "pp-test Module One",
      "id": 6000011
    },
    {
      "action": "create",
      "kind": "lesson",
      "path": "01-pp-test-module-one/01-lesson-one.md",
      "name": "pp-test Lesson One",
      "id": 5000042
    },
    {
      "action": "upload",
      "kind": "media",
      "path": "01-pp-test-module-one/01-lesson-one.md",
      "name": "pp-test Lesson One",
      "id": 5000042
    },
    {
      "action": "create",
      "kind": "lesson",
      "path": "01-pp-test-module-one/02-lesson-two.md",
      "name": "pp-test Lesson Two",
      "id": 5000043
    },
    {
      "action": "create",
      "kind": "section",
      "path": "02-pp-test-module-two",
      "name": "Pp Test Module Two",
      "id": 6000012
    },
    {
      "action": "create",
      "kind": "lesson",
      "path": "02-pp-test-module-two/01-lesson-three.md",
      "name": "pp-test Lesson Three",
      "id": 5000044
    },
    {
      "action": "reorder",
      "kind": "course",
      "path": "",
      "name": "",
      "id": 1001068
    }
  ],
  "summary": {
    "create": 5,
    "reorder": 1,
    "upload": 1
  }
}
## rerun preview (expect no creates)
{
  "applied": false,
  "space_id": 1001068,
  "plan": [
    {
      "action": "update",
      "kind": "lesson",
      "path": "01-pp-test-module-one/01-lesson-one.md",
      "name": "pp-test Lesson One",
      "id": 5000042
    },
    {
      "action": "update",
      "kind": "lesson",
      "path": "01-pp-test-module-one/02-lesson-two.md",
      "name": "pp-test Lesson Two",
      "id": 5000043
    },
    {
      "action": "update",
      "kind": "lesson",
      "path": "02-pp-test-module-two/01-lesson-three.md",
      "name": "pp-test Lesson Three",
      "id": 5000044
    }
  ],
  "summary": {
    "update": 3
  }
}
## pull
{
  "space_id": 1001068,
  "folder": "$TMPDIR/course-pulled",
  "files_written": 6,
  "note": "Lesson bodies retain HTML; media is not downloaded."
}
$TMPDIR/course-pulled/01-pp-test-module-one/_section.md
$TMPDIR/course-pulled/01-pp-test-module-one/01-pp-test-lesson-one.md
$TMPDIR/course-pulled/01-pp-test-module-one/02-pp-test-lesson-two.md
$TMPDIR/course-pulled/02-pp-test-module-two/_section.md
$TMPDIR/course-pulled/02-pp-test-module-two/01-pp-test-lesson-three.md
$TMPDIR/course-pulled/course.yaml
## rerun after normalization fix (expect noop)
{
  "applied": false,
  "space_id": 1001068,
  "plan": [],
  "summary": {
    "noop": 3
  }
}
## push the pulled folder (expect noop)
{
  "applied": false,
  "space_id": 1001068,
  "plan": [],
  "summary": {
    "noop": 3
  }
}
## edit one lesson then push --yes
{
  "applied": true,
  "space_id": 1001068,
  "plan": [
    {
      "action": "update",
      "kind": "lesson",
      "path": "01-pp-test-module-one/02-lesson-two.md",
      "name": "pp-test Lesson Two",
      "id": 5000043,
      "changes": [
        "body_html"
      ]
    }
  ],
  "summary": {
    "noop": 2,
    "update": 1
  }
}
<div><p>Second lesson body, edited.</p></div> draft
## reorder: lesson two renamed to 00- prefix, push --yes
{
  "applied": true,
  "space_id": 1001068,
  "plan": [
    {
      "action": "reorder",
      "kind": "course",
      "path": "",
      "name": "",
      "id": 1001068
    }
  ],
  "summary": {
    "noop": 3,
    "reorder": 1
  }
}
[(5000042, 'pp-test Lesson One', 6000011), (5000043, 'pp-test Lesson Two', 6000011), (5000044, 'pp-test Lesson Three', 6000012)]
after reorder rerun: {'noop': 3, 'reorder': 1}
## after order-ledger fix: push --yes (records order)
{
  "applied": true,
  "space_id": 1001068,
  "plan": [
    {
      "action": "reorder",
      "kind": "course",
      "path": "",
      "name": "",
      "id": 1001068
    }
  ],
  "summary": {
    "noop": 3,
    "reorder": 1
  }
}
## rerun (expect no reorder)
{
  "applied": false,
  "space_id": 1001068,
  "plan": [],
  "summary": {
    "noop": 3
  }
}
## move lesson two back to 02- (expect reorder only)
{
  "applied": false,
  "space_id": 1001068,
  "plan": [
    {
      "action": "reorder",
      "kind": "course",
      "path": "",
      "name": "",
      "id": 1001068
    }
  ],
  "summary": {
    "noop": 3,
    "reorder": 1
  }
}
{'noop': 3, 'reorder': 1}
## pull again (order note)
{
  "space_id": 1001068,
  "folder": "$TMPDIR/course-pull2",
  "files_written": 6,
  "note": "Circle's API does not expose lesson order; files are numbered in creation order. Lesson bodies retain HTML; media is not downloaded."
}
