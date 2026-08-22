# Go-to-Market & Pricing Design
**Date:** 2026-08-22
**Product:** go-bpmn-server — BPMN 2.0 workflow execution server (Go + PostgreSQL)
**Strategy:** Closed SaaS (Phase 1, months 0–12) → Open-Core SaaS (Phase 2, month 12+)

---

## 1. Positioning & Messaging

### Core problem
BPMN literacy is low in SMBs. Buyers do not search for "BPMN engine" — they search for "approval workflow software" or "process automation for small business". Leading with technical terminology loses 90% of the addressable market.

### Headline positioning
> **"Automate your internal processes without building a state machine."**
> Run structured approval workflows, onboarding sequences, and operational processes — defined visually, executed reliably, with full audit trail.

### What we are NOT
- Not another Zapier — we handle long-running, human-in-the-loop, standards-based processes
- Not enterprise BPM — lightweight, self-hostable, priced for real teams

### What we ARE
- The workflow engine for teams that have outgrown spreadsheets and Slack but don't need a $50K Camunda contract
- Self-hostable, Docker-native, REST API-first — your data stays yours
- Built on BPMN 2.0 — an open standard, not proprietary lock-in

### Primary buyer persona (Phase 1)
Operations Manager or IT Lead at a 50–150 person company. Feels the pain of manual processes. Has budget authority up to ~$500/month without escalating. Wants a demo, not a whitepaper.

### Secondary buyer persona (Phase 2, post-OSS)
Backend developer at a 10–50 person company who wants to embed a workflow engine in their product. Discovered via GitHub or Hacker News.

---

## 2. Product Requirements for Launch

### Must-have at launch (Month 0–3)

| Feature | Notes |
|---|---|
| **Visual BPMN Designer** | Embed bpmn.io (MIT licensed). Non-developers can draw and save process diagrams. Table stakes for the ops manager persona. |
| **Web Dashboard** | Running instances, completed instances, pending user tasks, incidents. Ops managers need visibility without hitting REST endpoints. |
| **User Task Inbox** | UI for humans to see tasks assigned to them, claim them, and complete them. Required for human-in-the-loop processes. |
| **Auth / User Management** | Email+password login, invite team members, role assignment (admin vs. participant). Use a managed auth provider (Clerk, Auth0, or Supabase Auth). |
| **Multi-tenancy** | Already built via `X-Tenant-ID`. The UI maps users to tenants via auth layer. |
| **Process Templates Library** | 5–10 pre-built BPMN diagrams: employee onboarding, purchase approval, IT access request, contract review, incident escalation. Eliminates blank-page problem. |
| **Hosted SaaS** | Clients do not run Docker themselves in Phase 1. Hosted on DigitalOcean/Railway/Fly.io. Self-hosting is an Enterprise upsell. |

### Nice-to-have (Month 3–6, after first clients)

- Email/Slack notifications when a user task is assigned or overdue
- SLA timers with alerting (process stuck >48h → notify)
- Process version management in the UI
- Webhook outbound for job workers
- Audit log export (CSV/JSON)
- SSO (SAML/OIDC) — required by larger SMBs, unlocks Pro tier

### Explicitly out of scope for launch

- Mobile app
- No-code integration marketplace
- AI-generated process diagrams
- On-premise self-hosted tier (added at Phase 2 OSS transition)

### Engineering delta from current state
The API server is solid. The gap is: **UI layer** (designer + dashboard + task inbox) + **auth** + **hosted infrastructure**.

---

## 3. Pricing Model

### Model: Per-workspace SaaS subscription
"Workspace" = one company/tenant. Maps naturally to the multi-tenant architecture. One bill per company, not per seat — seat pricing punishes adoption.

### Tiers

| Tier | Monthly | Annual | Limits | Key inclusions |
|---|---|---|---|---|
| **Starter** | $49/mo | $490/yr | 5 users, 3 active processes, 500 instances/mo | Visual designer, dashboard, task inbox, templates, community support |
| **Business** | $149/mo | $1,490/yr | 15 users, unlimited processes, 3,000 instances/mo | + Slack notifications, SLA alerts, audit log export, webhook outbound, email support |
| **Pro** | $399/mo | $3,990/yr | Unlimited users, unlimited processes, 15,000 instances/mo | + SSO, priority support, onboarding call |
| **Enterprise** | Custom | Custom | Unlimited + self-hosted | SLA contract, dedicated support, on-premise license |

### Pricing decisions

- **No permanent free tier in Phase 1.** Offer a 14-day free trial (no credit card) instead. Free tiers attract tire-kickers and create support overhead before infrastructure exists. Re-evaluate at Phase 2 OSS transition.
- **Annual discount: ~17% off.** Pitch annual as the default. Improves cash flow and reduces churn risk.
- **Self-hosted gated to Enterprise.** Data sovereignty is a real upsell for healthcare/finance. Opens up at Phase 2 as the OSS community tier.
- **Instance limits as the upgrade trigger.** Volume is a proxy for value delivered. Easier for clients to explain to finance than seat counts.

### Revenue targets

| Milestone | Composition | MRR |
|---|---|---|
| Ramen profitable (solo) | 15 × Starter | ~$735/mo |
| Sustainable (first hire) | 5 × Starter + 8 × Business | ~$1,440/mo |
| Real business | 10 × Business + 5 × Pro | ~$3,490/mo |

First 10 clients: 3–6 months with active outreach. First $3.5K MRR: 12-month target.

---

## 4. Go-to-Market Plan

### Month 0–2: Pre-launch — Design Partners

**Goal: 3 design partners (free access for 6 months in exchange for feedback + case study)**

**Channels:**
- Personal network first — anyone running a 20–150 person company with ops pain
- LinkedIn outreach: search "Operations Manager", "Process Manager", "Head of IT" at 50–200 person companies. 20–30 personalized messages/week.
- Script: *"We're building a workflow automation tool for internal processes — looking for 3 companies to use it free for 6 months in exchange for honest feedback. Would this be useful for [specific pain point]?"*
- Indie Hackers, Online Geniuses Slack, vertical Slacks (FinOps, HR Tech)

**Offer:** 6 months free, dedicated onboarding, logo on site, co-written case study.

---

### Month 2–6: Soft Launch + Direct Sales

**Goal: 10 paying clients**

**Launch activities:**
- Landing page: positioning, pricing, 14-day trial, 2–3 case study snippets
- Product Hunt launch (coordinate with design partners for upvotes)
- Hacker News "Show HN": frame as product story — *"Show HN: I built a lightweight BPMN workflow server in Go because Camunda was overkill"*
- Post in r/selfhosted, r/devops, r/golang

**Ongoing direct outreach (primary revenue driver):**
- LinkedIn: 30–50 targeted messages/week to ops managers and IT leads at 50–200 person companies
- Focus industries: legal, accounting, healthcare admin, logistics, property management, financial services
- Offer a free 30-minute "process audit" call — understand their worst manual process, show how it looks in the tool. Converts better than demos.

**G2 and Capterra:**
- Get 5+ reviews from design partners as early as possible. These platforms drive inbound for ops manager personas actively searching "workflow automation software".

---

### Month 6–12: Content + Inbound Layer

**Goal: 20 paying clients, first inbound leads**

**SEO content (high-intent, low competition):**
- "Camunda alternative for small business"
- "Self-hosted workflow automation"
- "Approval workflow software for [industry]"
- "BPMN process automation without Java"

**Template library (biggest conversion driver):**
- 10–15 free downloadable BPMN process templates, each as its own landing page
- Ops managers find templates via search → see live demo → sign up for trial

**Technical blog (one post/month):**
- Aimed at developers: "How we handle long-running processes in Go", "BPMN 2.0 vs Temporal: when to use each"
- Builds credibility for Phase 2 OSS transition and drives developer discovery

---

### What NOT to do in Phase 1

- No paid ads until conversion funnel is proven
- No broad social media content strategy
- No conference speaking
- No building integrations before clients ask

---

### Phase 2: Open-Core Transition (Month 12+)

**Trigger conditions:** 20+ paying clients, stable product, clear understanding of which features drive upgrades.

**What opens up:**
- Open-source the engine (`go-bpmn-engine`) and server (`go-bpmn-server`) under MIT
- Self-hosted tier becomes free (community) — drives GitHub stars and developer discovery
- SaaS UI, hosting, auth, and enterprise features remain proprietary
- Enterprise self-hosted becomes a paid managed deployment option

**OSS flywheel:** GitHub stars → developer discovery → self-hosters → some convert to paid cloud → some refer paying clients. This is the Camunda, n8n, and Temporal playbook.

---

## 5. Competitive Differentiation

### Defensible advantages

**Lightweight by design**
Single Go binary + PostgreSQL. Runs on a $6/month VPS. No JVM, no Elasticsearch, no cluster. Ops teams can self-host without a dedicated DevOps engineer.

**BPMN 2.0 standard — no vendor lock-in**
n8n, Make, Zapier, and Pipefy all use proprietary flow models. With BPMN 2.0, process diagrams are a portable open standard — importable into any BPMN-compatible tool. Trust signal for compliance-conscious SMBs.

**REST API-first**
Works with any language that can make an HTTP request. No SDK required. No framework dependency.

**Transparent SMB pricing**
Competitors either hit limits immediately (Zapier free) or hide behind a sales call (Camunda, Flowable, ProcessMaker). Self-serve pricing starting at $49/month with no "call us" wall.

**Data sovereignty**
SaaS-only tools mean process data (employee names, approval decisions, financial amounts) lives on vendor servers. Self-hosted Enterprise tier (and future OSS) lets regulated industries keep data on their own infrastructure.

---

### Head-to-head comparison

| | **go-bpmn-server** | Camunda | n8n | Zapier |
|---|---|---|---|---|
| BPMN 2.0 standard | ✅ | ✅ | ❌ | ❌ |
| Self-hostable | ✅ | ✅ (complex) | ✅ | ❌ |
| SMB pricing | ✅ $49/mo | ❌ $500+/mo | ✅ $20/mo | ✅ $20/mo |
| Human task management | ✅ | ✅ | ❌ | ❌ |
| Long-running processes | ✅ | ✅ | ❌ | ❌ |
| No JVM required | ✅ | ❌ | ✅ | ✅ |
| REST API-first | ✅ | ✅ | ✅ | ❌ |
| Visual designer | ✅ (bpmn.io) | ✅ | ✅ | ✅ |

---

### One-sentence pitches by persona

**Ops manager:**
> "Run your internal approval and onboarding processes with a visual workflow tool — no spreadsheets, no chasing people on Slack, full audit trail — at a price that doesn't need CFO approval."

**Developer:**
> "A REST API for BPMN 2.0 process execution, written in Go, runs on Docker + PostgreSQL, no JVM, no Zeebe cluster — just deploy and call endpoints."

**IT lead / compliance-focused:**
> "Your process data stays on your infrastructure. BPMN 2.0 is an open standard — no vendor lock-in, portable if you ever switch tools."

---

## 6. Execution Checklist

This section translates the strategy into a concrete week-by-week action plan with GitHub issue references. Each item has a clear done state.

---

### Milestone 1: Foundation (Month 0–2) — due 2026-10-22

**Goal:** Working hosted product + 3 design partners onboarded.

#### Engineering (parallel tracks)

| # | Issue | Done when |
|---|---|---|
| #9 | Builtin auth — PostgreSQL store + migrations | `AUTH_MODE=builtin` works end-to-end; login → JWT → API call succeeds |
| #10 | Builtin auth — admin CLI | `create-account`, `create-user`, `create-api-key` commands work |
| #11 | Hosted SaaS infrastructure | Product live at `https://<domain>`; auto-deploy from main |
| #12 | Web UI — BPMN designer | User can draw, save, and deploy a process from the browser |
| #13 | Web UI — process dashboard | Instances, incidents visible; cancel/suspend/resume actions work |
| #14 | Web UI — user task inbox | User can claim, complete, and unclaim tasks from the browser |
| #15 | Process templates (5 diagrams) | Templates selectable from "New process" picker; deploy and run cleanly |
| #17 | GET /processes endpoint | Process list page in UI populates correctly |
| #25 | README update | Auth, deployment, admin CLI all documented |
| #5  | History endpoint | GET/DELETE /instances/:id/history works |

#### GTM (starts week 3, after product is stable enough to demo)

| # | Issue | Done when |
|---|---|---|
| #18 | Design partner outreach | 3 signed agreements + intro calls done |

**Week-by-week:**
- **Week 1–2:** Infrastructure (#11) + auth PostgreSQL store (#9) — get the backend fully functional
- **Week 3–4:** Admin CLI (#10) + UI scaffold started (#12, #13, #14)
- **Week 5–6:** UI features complete — designer, dashboard, task inbox
- **Week 7–8:** Templates (#15) + endpoint (#17) + README (#25)
- **Week 6–8 (parallel):** Design partner outreach (#18) — start once you can demo a working product

---

### Milestone 2: Soft Launch (Month 2–6) — due 2027-02-22

**Goal:** 10 paying clients, $490–$735 MRR.

#### Engineering

| # | Issue | Done when |
|---|---|---|
| #16 | Billing — Stripe integration | Checkout, webhook, portal, and limit enforcement all work |
| #19 | Landing page | Live at domain, trial CTA working, Lighthouse >90 |

#### GTM

| # | Issue | Done when |
|---|---|---|
| #20 | G2 + Capterra listings | Profiles live with 5+ reviews each |
| #21 | Product Hunt launch | Post submitted; maker comment live; design partners briefed |
| #22 | Hacker News Show HN | Post submitted; top-level comments replied to |

**Step-by-step:**
1. **Month 2, week 1:** Landing page live (#19) + billing wired (#16) — required before any public launch
2. **Month 2, week 2:** Design partners leave G2/Capterra reviews (#20)
3. **Month 2–6 ongoing:** LinkedIn outreach 30–50 messages/week to ops managers (not tracked as an issue — recurring action)
4. **Month 3:** Product Hunt launch (#21) — requires G2 profile live and design partners briefed
5. **Month 3–4:** Hacker News Show HN (#22) — 2–3 weeks after Product Hunt to separate the signals

---

### Milestone 3: Growth (Month 6–12) — due 2027-08-22

**Goal:** 20 paying clients, first $3.5K MRR, inbound leads starting.

| # | Issue | Done when |
|---|---|---|
| #23 | SEO article — Camunda alternative | Published, indexed, ranking for target keywords |
| #24 | Template landing pages (5) | All 5 pages live, indexed, download + trial CTA working |

**Step-by-step:**
1. **Month 6:** Publish first SEO article (#23)
2. **Month 6:** Template landing pages live (#24)
3. **Month 6–12 ongoing:** One technical blog post per month (not tracked as an issue — recurring action)
4. **Month 12:** Evaluate Phase 2 OSS transition trigger conditions (20+ clients, stable product)

---

## 7. Key Risks & Mitigations

| Risk | Mitigation |
|---|---|
| BPMN literacy gap — SMBs can't use it without training | Embed bpmn.io designer + ship process templates + short "what is BPMN" doc section |
| Slow inbound before community exists | Direct outreach drives first 20 clients; inbound is Month 6+ |
| Camunda releases a lightweight SMB tier | Lean into Go/self-hosted/pricing advantages; their JVM dependency is structural, not fixable quickly |
| Design partners don't convert to paying | Qualify design partners on company size and ops pain before offering free access |
| Phase 2 OSS transition cannibalizes SaaS revenue | Gate all collaboration, hosting, and enterprise features behind SaaS; OSS is the engine only |
