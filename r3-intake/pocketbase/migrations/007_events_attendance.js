/// <reference path="../pb_data/types.d.ts" />
// 007_events_attendance.js — Events + Enrollment + Attendance collections.
// Requires: sites, intake, users collections (from 001_init.js).
// Locked rules (null) because Go is the policy layer (browser never touches PB).

migrate(
  (app) => {
    const intake = app.findCollectionByNameOrId("intake");
    const sites = app.findCollectionByNameOrId("sites");
    const users = app.findCollectionByNameOrId("users");

    // --- events collection ---
    let events = null;
    try { events = app.findCollectionByNameOrId("events"); } catch (e) {}
    if (!events) {
      events = new Collection({
        name: "events",
        type: "base",
        listRule: null,
        viewRule: null,
        createRule: null,
        updateRule: null,
        deleteRule: null,
        fields: [
          { name: "site", type: "relation", collectionId: sites.id, maxSelect: 1, required: true, cascadeDelete: false },
          { name: "name", type: "text", required: true, max: 200 },
          { name: "start_date", type: "text", required: true, max: 20 },
          { name: "end_date", type: "text", required: true, max: 20 },
          { name: "description", type: "text", required: false, max: 500 },
          { name: "status", type: "select", required: true, values: ["active", "completed", "cancelled"], maxSelect: 1 },
          { name: "created_by", type: "relation", collectionId: users.id, maxSelect: 1, required: false, cascadeDelete: false },
          { name: "created", type: "autodate", onCreate: true, onUpdate: false },
          { name: "updated", type: "autodate", onCreate: false, onUpdate: true }
        ]
      });
      app.save(events);
    }

    // --- event_enrollment junction collection ---
    let enrollment = null;
    try { enrollment = app.findCollectionByNameOrId("event_enrollment"); } catch (e) {}
    if (!enrollment) {
      enrollment = new Collection({
        name: "event_enrollment",
        type: "base",
        listRule: null,
        viewRule: null,
        createRule: null,
        updateRule: null,
        deleteRule: null,
        fields: [
          { name: "event", type: "relation", collectionId: events.id, maxSelect: 1, required: true, cascadeDelete: true },
          { name: "intake", type: "relation", collectionId: intake.id, maxSelect: 1, required: true, cascadeDelete: true },
          { name: "enrolled_date", type: "text", required: false, max: 20 },
          { name: "created", type: "autodate", onCreate: true, onUpdate: false }
        ]
      });
      app.save(enrollment);
    }

    // --- attendance collection (event is nullable) ---
    let attendance = null;
    try { attendance = app.findCollectionByNameOrId("attendance"); } catch (e) {}
    if (!attendance) {
      attendance = new Collection({
        name: "attendance",
        type: "base",
        listRule: null,
        viewRule: null,
        createRule: null,
        updateRule: null,
        deleteRule: null,
        fields: [
          { name: "event", type: "relation", collectionId: events.id, maxSelect: 1, required: false, cascadeDelete: true },
          { name: "intake", type: "relation", collectionId: intake.id, maxSelect: 1, required: true, cascadeDelete: true },
          { name: "site", type: "relation", collectionId: sites.id, maxSelect: 1, required: true, cascadeDelete: false },
          { name: "recorded_by", type: "relation", collectionId: users.id, maxSelect: 1, required: false, cascadeDelete: false },
          { name: "date", type: "text", required: true, max: 20 },
          { name: "status", type: "select", required: true, values: ["present", "absent", "excused", "walk_in"], maxSelect: 1 },
          { name: "check_in_time", type: "text", required: false, max: 20 },
          { name: "note", type: "text", required: false, max: 500 },
          { name: "created", type: "autodate", onCreate: true, onUpdate: false },
          { name: "updated", type: "autodate", onCreate: false, onUpdate: true }
        ]
      });
      app.save(attendance);
    }
  },
  (app) => {
    // Down: drop in reverse dependency order
    for (const name of ["attendance", "event_enrollment", "events"]) {
      let col; try { col = app.findCollectionByNameOrId(name); } catch (e) {}
      if (col) app.delete(col);
    }
  }
);
