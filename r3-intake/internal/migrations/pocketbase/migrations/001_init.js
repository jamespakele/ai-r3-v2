/// <reference path="../pb_data/types.d.ts" />
// 001_init.js — PocketBase schema for the R3 Intake app (PocketBase v0.39 JS API).
//
// Creates: sites, intake, and adds role/name to the built-in users collection.
// Seeds the two reference sites. Executed by the jsvm plugin during bootstrap
// (v0.39 auto-runs pending migrations on start).
//
// v0.39 JS migration API: `app` is a core.App exposed with camelCase method
// names (findCollectionByNameOrId, save, delete, findRecordsByFilter). There is
// no app.dao(). Fields are plain JSON objects with a `type` discriminator.
//
// All collection API rules are locked to admin/superuser because the Go server
// is the policy layer and performs every collection op via the embedded
// core.App in-process — the browser never talks to PocketBase directly.

migrate(
  (app) => {
    // --- sites --------------------------------------------------------------
    let sites = null;
    try { sites = app.findCollectionByNameOrId("sites"); } catch (e) {}
    if (!sites) {
      sites = new Collection({
        name: "sites",
        type: "base",
        listRule: null,
        viewRule: null,
        createRule: null,
        updateRule: null,
        deleteRule: null,
        fields: [
          { name: "name", type: "text", required: true, min: 1, max: 120 },
          { name: "address", type: "text", required: false, max: 240 },
          { name: "active", type: "bool", required: false },
          { name: "sort_order", type: "number", required: false, onlyInt: true }
        ],
      });
      app.save(sites);
    }

    // Seed the two reference sites (idempotent).
    const existing = app.findRecordsByFilter("sites", "1=1", "sort_order", 100, 0);
    if (!existing || existing.length === 0) {
      const seeds = [
        { name: "Kapolei Library", address: "91-1004 Kapolei Pkwy, Kapolei, HI 96707", sort_order: 1 },
        { name: "Paradise Chapel of the Assemblies of God", address: "87-212 Farrington Hwy, Wai\u02bbanae, HI 96792", sort_order: 2 },
      ];
      for (const s of seeds) {
        const rec = new Record(sites, {
          name: s.name,
          address: s.address,
          active: true,
          sort_order: s.sort_order,
        });
        app.save(rec);
      }
    }

    // --- users (auth) — add role + name ------------------------------------
    let users = null;
    try { users = app.findCollectionByNameOrId("users"); } catch (e) {}
    if (!users) {
      users = new Collection({
        name: "users",
        type: "auth",
        listRule: 'id = @request.auth.id || @request.auth.role = "admin"',
        viewRule: 'id = @request.auth.id || @request.auth.role = "admin"',
        createRule: null,
        updateRule: 'id = @request.auth.id || @request.auth.role = "admin"',
        deleteRule: null,
        fields: [
          { name: "name", type: "text", required: false, max: 120 },
          { name: "role", type: "select", required: true, values: ["admin", "case_manager"], maxSelect: 1 }
        ],
      });
      app.save(users);
    } else {
      let dirty = false;
      if (!users.fields.getByName("name")) {
        users.fields.add(new Field({ name: "name", type: "text", required: false, max: 120 }));
        dirty = true;
      }
      if (!users.fields.getByName("role")) {
        users.fields.add(new Field({ name: "role", type: "select", required: true, values: ["admin", "case_manager"], maxSelect: 1 }));
        dirty = true;
      }
      if (dirty) app.save(users);
    }

    let intake = null;
    try { intake = app.findCollectionByNameOrId("intake"); } catch (e) {}
    if (!intake) {
      intake = new Collection({
        name: "intake",
        type: "base",
        // Go performs all ops in-process as a superuser (bypasses rules). The
        // public rules stay locked so the browser can never touch PB directly.
        listRule: '@request.auth.role = "admin"',
        viewRule: '@request.auth.role = "admin"',
        createRule: null,
        updateRule: null,
        deleteRule: null,
        fields: [
          // Section 01 — Personal Profile
          { name: "site", type: "relation", collectionId: sites.id, maxSelect: 1, required: false, cascadeDelete: false },
          { name: "name", type: "text", required: false, max: 200 },
          { name: "dob", type: "text", required: false, max: 20 }, // sensitive
          { name: "ssn", type: "text", required: false, max: 20 }, // sensitive
          { name: "contact", type: "text", required: false, max: 60 },
          { name: "email", type: "email", required: false },
          { name: "livingWith", type: "text", required: false, max: 20 },
          { name: "household", type: "json", required: false, maxSize: 20000 },
          { name: "sleptWhere", type: "text", required: false, max: 300 },
          { name: "race", type: "json", required: false, maxSize: 2000 },
          { name: "raceOther", type: "text", required: false, max: 120 },
          { name: "sexAtBirth", type: "text", required: false, max: 20 },
          { name: "servedMilitary", type: "text", required: false, max: 20 },
          { name: "militaryDetail", type: "text", required: false, max: 120 },
          { name: "hasPets", type: "text", required: false, max: 20 },
          { name: "petSupport", type: "text", required: false, max: 20 },
          { name: "petPrevented", type: "text", required: false, max: 20 },
          { name: "employment", type: "text", required: false, max: 20 },
          { name: "unemployedDuration", type: "text", required: false, max: 20 },
          { name: "interestedEmployed", type: "text", required: false, max: 20 },
          { name: "jobTypes", type: "text", required: false, max: 300 },
          // Section 02 — Medical History / Health & safety
          { name: "mentalHealth", type: "text", required: false, max: 20 },
          { name: "substanceUse", type: "text", required: false, max: 20 },
          { name: "fleeingViolence", type: "text", required: false, max: 20 },
          { name: "homelessFactors", type: "text", required: false, max: 4000 },
          // Section 03 — Homeless Verification / Documentation on file
          { name: "hmis", type: "bool", required: false },
          { name: "hmisProvider", type: "text", required: false, max: 120 },
          { name: "documents", type: "json", required: false, maxSize: 2000 },
          { name: "healthInsuranceDetail", type: "text", required: false, max: 120 },
          { name: "housing", type: "json", required: false, maxSize: 2000 },
          { name: "income", type: "json", required: false, maxSize: 2000 },
          // Section 04 — Personal
          { name: "personal", type: "json", required: false, maxSize: 20000 },
          // Section 05 — Service Plan
          { name: "servicePlan", type: "json", required: false, maxSize: 20000 },
          // Section 06 — Signatures (text columns, PNG data URLs)
          { name: "participantName", type: "text", required: false, max: 200 },
          { name: "participantSigTyped", type: "text", required: false, max: 200 },
          { name: "participantSigDataUrl", type: "text", required: false, max: 100000 }, // sensitive
          { name: "casemanagerName", type: "text", required: false, max: 200 },
          { name: "casemanagerSigTyped", type: "text", required: false, max: 200 },
          { name: "casemanagerSigDataUrl", type: "text", required: false, max: 100000 }, // sensitive
          // Workflow
          { name: "status", type: "select", required: false, values: ["unassigned", "claimed", "completed"], maxSelect: 1 },
          { name: "assigned_to", type: "relation", collectionId: users.id, maxSelect: 1, required: false, cascadeDelete: false },
          { name: "created_by", type: "relation", collectionId: users.id, maxSelect: 1, required: false, cascadeDelete: false },
          { name: "created", type: "autodate", onCreate: true, onUpdate: false },
          { name: "updated", type: "autodate", onCreate: false, onUpdate: true }
        ],
      });
      app.save(intake);
    }
  },
  (app) => {
    // Down — drop the collections we created (users role/name fields left).
    for (const name of ["intake", "sites"]) {
      let col = null;
      try { col = app.findCollectionByNameOrId(name); } catch (e) {}
      if (col) app.delete(col);
    }
  }
);