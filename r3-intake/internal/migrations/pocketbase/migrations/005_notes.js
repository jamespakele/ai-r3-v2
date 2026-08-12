/// <reference path="../pb_data/types.d.ts" />
// 005_notes.js — per-participant notes (one intake → many notes).
// Relations reference the intake + users collections created by 001_init.js.
// Locked rules (null) because Go is the policy layer (browser never touches PB).

migrate(
  (app) => {
    const intake = app.findCollectionByNameOrId("intake");
    const users = app.findCollectionByNameOrId("users");

    let notes = null;
    try { notes = app.findCollectionByNameOrId("notes"); } catch (e) {}
    if (!notes) {
      notes = new Collection({
        name: "notes",
        type: "base",
        listRule: null,
        viewRule: null,
        createRule: null,
        updateRule: null,
        deleteRule: null,
        fields: [
          { name: "intake", type: "relation", collectionId: intake.id, maxSelect: 1, required: true, cascadeDelete: true },
          { name: "author", type: "relation", collectionId: users.id, maxSelect: 1, required: true, cascadeDelete: false },
          { name: "date", type: "text", required: false, max: 20 },
          { name: "body", type: "text", required: true, max: 20000 },
          { name: "created", type: "autodate", onCreate: true, onUpdate: false },
          { name: "updated", type: "autodate", onCreate: false, onUpdate: true }
        ]
      });
      app.save(notes);
    }
  },
  (app) => {
    let notes = null;
    try { notes = app.findCollectionByNameOrId("notes"); } catch (e) {}
    if (notes) app.delete(notes);
  }
);
