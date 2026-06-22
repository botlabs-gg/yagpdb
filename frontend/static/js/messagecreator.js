// NOTE: the control panel loads pages via partial AJAX navigation ($("#main-content").html(...)),
// so DOMContentLoaded does NOT fire on navigation. We initialise immediately and scope every
// listener to #mc-root. Toggles are driven by explicit label clicks (Bootstrap's
// data-toggle="buttons" fires a jQuery-only change event native listeners never receive).
(function () {
    "use strict";

    let BUTTON_STYLES = {
        1: { bg: "#5865f2", fg: "#fff" },
        2: { bg: "#4f545c", fg: "#fff" },
        3: { bg: "#248046", fg: "#fff" },
        4: { bg: "#da373c", fg: "#fff" },
        5: { bg: "#4f545c", fg: "#fff" }
    };

    let NODE_TITLES = { 1: "Action Row", 9: "Section", 10: "Text", 12: "Media Gallery", 14: "Separator", 17: "Container" };

    let TEMPLATE_PREFIX = "templates-";
    let MAX_CUSTOM_ID = 100;                                 // Discord limit, including the prefix
    let MAX_SUFFIX = MAX_CUSTOM_ID - TEMPLATE_PREFIX.length; // editable part the user types
    let PRESET_COLORS = ["#5865f2", "#57f287", "#fee75c", "#eb459e", "#ed4245", "#1abc9c",
        "#3498db", "#9b59b6", "#e67e22", "#f1c40f", "#95a5a6", "#ffffff", "#2c2f33", "#000000"];

    let currentMode = "normal";
    let currentAction = "create";
    let loadedForEdit = false;
    let state = { embeds: [], components: [] };
    let els = {};

    function init() {
        let root = document.getElementById("mc-root");
        if (!root) return;

        els.root = root;
        els.guildID = root.getAttribute("data-guildid");
        els.form = document.getElementById("mc-form");
        els.normalFields = document.getElementById("normal-fields");
        els.componentsLabel = document.getElementById("components-label");
        els.builder = document.getElementById("components-builder");
        els.addBar = document.getElementById("components-add-bar");
        els.modeHint = document.getElementById("mc-mode-hint");
        els.errors = document.getElementById("mc-errors");
        els.preview = document.getElementById("mc-preview");
        els.compError = document.getElementById("components-error");
        els.embedsBuilder = document.getElementById("embeds-builder");
        els.embedsAddBar = document.getElementById("embeds-add-bar");
        els.submit = document.getElementById("mc-submit");
        els.submitLabel = document.getElementById("mc-submit-label");
        els.submitIcon = document.getElementById("mc-submit-icon");
        els.editHint = document.getElementById("mc-edit-hint");
        els.typeLockNote = document.getElementById("mc-type-lock-note");

        root.querySelectorAll(".mc-action-toggle label").forEach(function (label) {
            label.addEventListener("click", function (e) {
                e.preventDefault();
                let input = label.querySelector("input");
                if (input) setAction(input.value);
            });
        });
        root.querySelectorAll(".mc-mode-toggle label").forEach(function (label) {
            label.addEventListener("click", function (e) {
                e.preventDefault();
                if (label.classList.contains("disabled")) return;
                let input = label.querySelector("input");
                if (input) setMode(input.value);
            });
        });

        root.addEventListener("input", function (e) {
            if (e.target.classList && (e.target.classList.contains("mc-input") || e.target.classList.contains("mc-field-input"))) {
                renderPreview();
            }
        });

        let loadBtn = document.getElementById("mc-load-btn");
        if (loadBtn) loadBtn.addEventListener("click", loadMessage);

        els.form.addEventListener("submit", onSubmit);

        setAction("create");
        setMode("normal");
        renderEmbeds();
    }

    function base() { return "/manage/" + els.guildID + "/messagecreator"; }

    function syncToggle(name) {
        els.root.querySelectorAll("input[name=" + name + "]").forEach(function (r) {
            let label = r.closest("label");
            if (label) label.classList.toggle("active", r.checked);
        });
    }

    function setAction(action) {
        currentAction = action;
        let creating = action === "create";
        els.root.querySelectorAll("input[name=mc-action]").forEach(function (r) { r.checked = r.value === action; });
        syncToggle("mc-action");

        show(els.root.querySelector(".mc-when-create"), creating);
        show(els.root.querySelector(".mc-when-existing"), !creating);

        if (creating) {
            els.submit.className = "btn btn-success btn-block";
            els.submitLabel.textContent = "Create message";
            els.submit.disabled = false;
            show(els.editHint, false);
            setTypeLock(null);
        } else {
            els.submit.className = "btn btn-warning btn-block";
            els.submitLabel.textContent = "Edit Message";
            els.submit.disabled = !loadedForEdit;
            show(els.editHint, !loadedForEdit);
        }
    }

    function setMode(mode) {
        currentMode = mode;
        els.root.querySelectorAll("input[name=mc-mode]").forEach(function (r) { r.checked = r.value === mode; });
        syncToggle("mc-mode");

        let v2 = mode === "componentsv2";
        show(els.normalFields, !v2);
        els.componentsLabel.innerHTML = v2
            ? "Components"
            : 'Components <span class="text-muted">(optional — buttons &amp; menus)</span>';
        els.modeHint.textContent = v2
            ? "Components V2: content & embeds are replaced by components (Text, Container, Section, …)."
            : "Normal message: content, an embed, and up to 5 rows of buttons / select menus.";

        renderBuilder();
        renderPreview();
    }

    function setTypeLock(lockedMode) {
        let radios = els.root.querySelectorAll("input[name=mc-mode]");
        if (lockedMode) {
            setMode(lockedMode);
            radios.forEach(function (r) {
                let label = r.closest("label");
                if (label) label.classList.toggle("disabled", r.value !== lockedMode);
            });
            els.typeLockNote.style.display = "";
            els.typeLockNote.textContent = lockedMode === "componentsv2"
                ? "This is a Components V2 message — its type can't be changed when editing."
                : "Editing an existing message — its type can't be changed.";
        } else {
            radios.forEach(function (r) {
                let label = r.closest("label");
                if (label) label.classList.remove("disabled");
            });
            els.typeLockNote.style.display = "none";
        }
    }

    function onSubmit(e) {
        els.form.action = base() + "/" + (currentAction === "create" ? "send" : "edit");
        let msg = computeMessage().message;
        let errs = validateComposed(msg, currentMode);
        if (errs.length) {
            e.preventDefault(); e.stopPropagation();
            showErrors(errs);
            return;
        }
        clearErrors();
        document.getElementById("mc-f-mode").value = currentMode;
        document.getElementById("mc-f-payload").value = JSON.stringify(msg);
    }

    function showErrors(errs) {
        els.errors.innerHTML = "<strong>Please fix the following before sending:</strong>" +
            "<ul class='mb-0 mt-1'>" + errs.map(function (s) { return "<li>" + escapeHtml(s) + "</li>"; }).join("") + "</ul>";
        els.errors.style.display = "";
        els.errors.scrollIntoView({ behavior: "smooth", block: "center" });
    }
    function clearErrors() { els.errors.style.display = "none"; els.errors.innerHTML = ""; }

    function charLen(s) { return s ? String(s).length : 0; }
    function isURL(s) { return /^https?:\/\//i.test(s) || /^attachment:\/\//i.test(s); }

    function countAllComponents(comps) {
        let n = 0;
        (comps || []).forEach(function (c) {
            n++;
            if (c.components) n += countAllComponents(c.components);
            if (c.accessory) n++;
        });
        return n;
    }

    function validateComposed(msg, mode) {
        let errs = [];
        if (mode === "normal") {
            if (charLen(msg.content) > 2000) errs.push("Message content exceeds 2000 characters.");
            let embeds = msg.embeds || [];
            if (embeds.length > 10) errs.push("A message can have at most 10 embeds.");
            embeds.forEach(function (e, i) { validateEmbed(e, i + 1, errs); });
            if (!msg.content && !embeds.length && !(msg.components || []).length) {
                errs.push("Add content, an embed, or a component before sending.");
            }
            let rows = (msg.components || []).filter(function (c) { return c.type === 1; }).length;
            if (rows > 5) errs.push("A message can have at most 5 action rows.");
        } else {
            if (!(msg.components || []).length) errs.push("Add at least one component for a Components V2 message.");
            if (countAllComponents(msg.components) > 40) errs.push("A Components V2 message can have at most 40 components in total.");
        }
        validateComponents(msg.components || [], errs);
        return errs;
    }

    function validateEmbed(e, n, errs) {
        let total = 0;
        if (charLen(e.title) > 256) errs.push("Embed " + n + ": title exceeds 256 characters.");
        if (charLen(e.description) > 4096) errs.push("Embed " + n + ": description exceeds 4096 characters.");
        total += charLen(e.title) + charLen(e.description);
        if (e.author) { if (charLen(e.author.name) > 256) errs.push("Embed " + n + ": author name exceeds 256 characters."); total += charLen(e.author.name); }
        if (e.footer) { if (charLen(e.footer.text) > 2048) errs.push("Embed " + n + ": footer text exceeds 2048 characters."); total += charLen(e.footer.text); }
        let fields = e.fields || [];
        if (fields.length > 25) errs.push("Embed " + n + ": at most 25 fields.");
        fields.forEach(function (f, fi) {
            if (!f.name || !f.value) errs.push("Embed " + n + " field " + (fi + 1) + ": both name and value are required.");
            if (charLen(f.name) > 256) errs.push("Embed " + n + " field " + (fi + 1) + ": name exceeds 256 characters.");
            if (charLen(f.value) > 1024) errs.push("Embed " + n + " field " + (fi + 1) + ": value exceeds 1024 characters.");
            total += charLen(f.name) + charLen(f.value);
        });
        if (total > 6000) errs.push("Embed " + n + ": total text exceeds 6000 characters.");
        [e.url, e.image && e.image.url, e.thumbnail && e.thumbnail.url, e.author && e.author.icon_url, e.footer && e.footer.icon_url]
            .forEach(function (u) { if (u && !isURL(u)) errs.push("Embed " + n + ": \"" + u + "\" is not a valid URL (must start with http:// or https://)."); });
    }

    function validateComponents(comps, errs) {
        comps.forEach(function (c) {
            switch (c.type) {
                case 1:
                    let items = c.components || [];
                    if (!items.length) errs.push("An action row is empty — add a button or select menu, or remove it.");
                    if (items.length > 5) errs.push("An action row can have at most 5 components.");
                    if (items.some(function (x) { return x.type !== 2; }) && items.length > 1) {
                        errs.push("A select menu can't share an action row with other components.");
                    }
                    items.forEach(function (x) { validateInteractive(x, errs); });
                    break;
                case 17:
                    if (!(c.components || []).length) errs.push("A container must contain at least one component.");
                    validateComponents(c.components || [], errs);
                    break;
                case 9:
                    if (!(c.components || []).some(function (t) { return t.type === 10 && t.content; })) errs.push("A section needs at least one non-empty text component.");
                    if (!c.accessory) errs.push("A section needs an accessory (button or thumbnail).");
                    else if (c.accessory.type === 2) validateInteractive(c.accessory, errs);
                    else if (c.accessory.type === 11 && !(c.accessory.media && c.accessory.media.url)) errs.push("A section thumbnail needs an image URL.");
                    break;
                case 10:
                    if (!c.content) errs.push("A text component can't be empty.");
                    if (charLen(c.content) > 4000) errs.push("A text component exceeds 4000 characters.");
                    break;
                case 12:
                    if (!(c.items || []).length) errs.push("A media gallery needs at least one item.");
                    (c.items || []).forEach(function (it) {
                        if (!(it.media && it.media.url)) errs.push("A media gallery item needs an image URL.");
                        else if (!isURL(it.media.url)) errs.push("A media gallery URL must start with http:// or https://.");
                    });
                    break;
            }
        });
    }

    function validateInteractive(c, errs) {
        if (c.type === 2) {
            if (c.style === 5) {
                if (!c.url) errs.push("A link button needs a URL.");
                else if (!isURL(c.url)) errs.push("A link button URL must start with http:// or https://.");
            } else if (!c.label && !(c.emoji && c.emoji.name)) {
                errs.push("A button needs a label or emoji.");
            }
            if (charLen(c.label) > 80) errs.push("A button label exceeds 80 characters.");
            if (c.custom_id && charLen(c.custom_id) > MAX_CUSTOM_ID) errs.push("A button custom id is too long (max " + MAX_SUFFIX + " characters, plus the templates- prefix).");
        } else {
            if (charLen(c.placeholder) > 150) errs.push("A select placeholder exceeds 150 characters.");
            if (c.custom_id && charLen(c.custom_id) > MAX_CUSTOM_ID) errs.push("A select custom id is too long (max " + MAX_SUFFIX + " characters, plus the templates- prefix).");
            if (!c.type || c.type === 3) {
                let opts = c.options || [];
                if (opts.length < 1 || opts.length > 25) errs.push("A text-options select menu needs between 1 and 25 options.");
                let seen = {};
                opts.forEach(function (o, oi) {
                    if (!o.label) errs.push("Select option " + (oi + 1) + " needs a label.");
                    if (!o.value) errs.push("Select option " + (oi + 1) + " needs a value.");
                    else { if (seen[o.value]) errs.push("Select option values must be unique."); seen[o.value] = true; }
                });
            }
            if (c.min_values != null && (c.min_values < 0 || c.min_values > 25)) errs.push("Select min values must be between 0 and 25.");
            if (c.max_values != null && (c.max_values < 1 || c.max_values > 25)) errs.push("Select max values must be between 1 and 25.");
            if (c.min_values != null && c.max_values != null && c.min_values > c.max_values) errs.push("Select min values can't exceed max values.");
        }
    }

    function escapeHtml(s) { return String(s).replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;").replace(/"/g, "&quot;"); }


    function fieldValue(id) { let el = document.getElementById(id); return el ? el.value.trim() : ""; }

    function defEmbed() {
        return {
            title: "", url: "", description: "", color: undefined,
            authorName: "", authorIcon: "", footerText: "", footerIcon: "",
            imageUrl: "", thumbUrl: "", ts: false, fields: []
        };
    }

    function renderEmbeds() {
        if (!els.embedsBuilder) return;
        els.embedsBuilder.innerHTML = "";
        state.embeds.forEach(function (e, i) { els.embedsBuilder.appendChild(embedCard(e, i)); });
        els.embedsAddBar.innerHTML = "";
        if (state.embeds.length < 10) {
            els.embedsAddBar.appendChild(btn('<i class="fas fa-plus"></i> Add embed', "btn-block btn-outline-primary", function () {
                state.embeds.push(defEmbed()); renderEmbeds(); renderPreview();
            }));
        }
    }

    function embedCard(e, idx) {
        let card = el("div", { class: "mc-node" });
        card.appendChild(nodeHeader("Embed " + (idx + 1), state.embeds, idx, rebuildEmbeds));

        card.appendChild(el("div", { class: "form-row" }, [
            el("div", { class: "col-md-8" }, [field("Title", e.title, function (v) { e.title = v; refreshPreview(); })]),
            el("div", { class: "col-md-4" }, [colorField("Color", e.color, function (v) { e.color = v; refreshPreview(); })])
        ]));
        card.appendChild(field("Title URL", e.url, function (v) { e.url = v; refreshPreview(); }, { placeholder: "https://..." }));
        card.appendChild(field("Description", e.description, function (v) { e.description = v; refreshPreview(); }, { textarea: true, rows: 3 }));
        card.appendChild(el("div", { class: "form-row" }, [
            el("div", { class: "col-md-6" }, [field("Image URL", e.imageUrl, function (v) { e.imageUrl = v; refreshPreview(); }, { placeholder: "https://..." })]),
            el("div", { class: "col-md-6" }, [field("Thumbnail URL", e.thumbUrl, function (v) { e.thumbUrl = v; refreshPreview(); }, { placeholder: "https://..." })])
        ]));
        card.appendChild(el("div", { class: "form-row" }, [
            el("div", { class: "col-md-6" }, [field("Author name", e.authorName, function (v) { e.authorName = v; refreshPreview(); })]),
            el("div", { class: "col-md-6" }, [field("Author icon URL", e.authorIcon, function (v) { e.authorIcon = v; refreshPreview(); }, { placeholder: "https://..." })])
        ]));
        card.appendChild(el("div", { class: "form-row" }, [
            el("div", { class: "col-md-8" }, [field("Footer text", e.footerText, function (v) { e.footerText = v; refreshPreview(); })]),
            el("div", { class: "col-md-4" }, [field("Footer icon URL", e.footerIcon, function (v) { e.footerIcon = v; refreshPreview(); }, { placeholder: "https://..." })])
        ]));
        let tsCb = el("input", { type: "checkbox" }); tsCb.className = "mr-2"; tsCb.checked = !!e.ts;
        tsCb.addEventListener("change", function () { e.ts = tsCb.checked; refreshPreview(); });
        card.appendChild(el("label", { class: "d-flex align-items-center mb-2" }, [tsCb, "Include current timestamp in footer"]));

        card.appendChild(el("hr", { class: "my-2" }));
        card.appendChild(el("label", { class: "small d-block mb-1" }, ["Fields"]));
        e.fields = e.fields || [];
        e.fields.forEach(function (f, fi) {
            let inlineCb = el("input", { type: "checkbox" }); inlineCb.checked = !!f.inline;
            inlineCb.addEventListener("change", function () { f.inline = inlineCb.checked; refreshPreview(); });
            card.appendChild(el("div", { class: "mc-opt-row" }, [
                inlineInput("Field name", f.name, function (v) { f.name = v; refreshPreview(); }),
                inlineInput("Field value", f.value, function (v) { f.value = v; refreshPreview(); }),
                el("label", { class: "mb-0 small text-nowrap", title: "Display inline" }, [inlineCb, " in"]),
                btn('<i class="fas fa-times"></i>', "btn-block btn-danger", function () { e.fields.splice(fi, 1); renderEmbeds(); renderPreview(); }, { noconfirm: true })
            ]));
        });
        if (e.fields.length < 25) {
            card.appendChild(btn("+ Field", "btn-block btn-outline-secondary", function () { e.fields.push({ name: "", value: "", inline: false }); renderEmbeds(); renderPreview(); }));
        }
        return card;
    }

    // Build clean Discord embed objects from the model, dropping empty ones.
    function buildEmbeds() {
        let out = [];
        state.embeds.forEach(function (e) {
            let emb = {};
            if (e.title) emb.title = e.title;
            if (e.url) emb.url = e.url;
            if (e.description) emb.description = e.description;
            if (typeof e.color === "number") emb.color = e.color;
            if (e.authorName || e.authorIcon) { emb.author = {}; if (e.authorName) emb.author.name = e.authorName; if (e.authorIcon) emb.author.icon_url = e.authorIcon; }
            if (e.footerText || e.footerIcon) { emb.footer = {}; if (e.footerText) emb.footer.text = e.footerText; if (e.footerIcon) emb.footer.icon_url = e.footerIcon; }
            if (e.imageUrl) emb.image = { url: e.imageUrl };
            if (e.thumbUrl) emb.thumbnail = { url: e.thumbUrl };
            if (e.ts) emb.timestamp = new Date().toISOString();
            let fields = (e.fields || []).filter(function (f) { return f.name || f.value; })
                .map(function (f) { return { name: f.name || "", value: f.value || "", inline: !!f.inline }; });
            if (fields.length) emb.fields = fields;
            if (Object.keys(emb).length) out.push(emb);
        });
        return out;
    }

    // A checkbox + color picker bound to an optional numeric color (undefined = no color).
    function colorPicker(value, oninput) {
        let cur = (typeof value === "number") ? intToHex(value) : "#5865f2";
        let swatch = el("span", { class: "mc-color-swatch" }); swatch.style.background = cur;
        let hexIn = el("input", { class: "form-control form-control-sm", maxlength: "7" });
        hexIn.value = cur; hexIn.style.maxWidth = "110px";
        function apply(h, fromHexInput) {
            h = h.trim(); if (h[0] !== "#") h = "#" + h;
            if (!/^#[0-9a-fA-F]{6}$/.test(h)) return;
            swatch.style.background = h;
            if (!fromHexInput) hexIn.value = h;
            oninput(hexToInt(h));
        }
        hexIn.addEventListener("input", function () { apply(hexIn.value, true); });
        let palette = el("div", { class: "mc-color-palette" });
        PRESET_COLORS.forEach(function (c) {
            let s = el("span", { class: "mc-color-swatch mc-color-preset" });
            s.style.background = c; s.title = c;
            s.addEventListener("click", function () { apply(c, false); });
            palette.appendChild(s);
        });
        return el("div", {}, [el("div", { class: "d-flex align-items-center", style: "gap:6px;" }, [swatch, hexIn]), palette]);
    }

    // Optional-color field: a checkbox enables the color picker; unchecked means no color.
    function colorField(labelText, value, oninput) {
        let selected = (typeof value === "number") ? value : hexToInt("#5865f2");
        let enabled = typeof value === "number";
        let cb = el("input", { type: "checkbox" }); cb.className = "mr-2"; cb.checked = enabled;
        let pickerWrap = el("div", { class: "mt-1" }, [colorPicker(value, function (v) { selected = v; if (cb.checked) oninput(v); })]);
        show(pickerWrap, enabled);
        cb.addEventListener("change", function () { show(pickerWrap, cb.checked); oninput(cb.checked ? selected : undefined); });
        return el("div", { class: "form-group mb-2" }, [
            el("label", { class: "small mb-1 d-block" }, [cb, labelText]),
            pickerWrap
        ]);
    }

    function showCompError(msg) { if (els.compError) els.compError.textContent = msg || ""; }


    // refreshPreview: a field changed — refresh the preview without rebuilding the editor (keeps
    // input focus). rebuildEditor: structure changed — re-render the whole builder.
    function refreshPreview() { renderPreview(); }
    function rebuildEditor() { renderBuilder(); renderPreview(); }

    function rebuildEmbeds() { renderEmbeds(); renderPreview(); }

    function move(list, idx, delta, onChange) {
        let j = idx + delta;
        if (j < 0 || j >= list.length) return;
        let tmp = list[idx]; list[idx] = list[j]; list[j] = tmp;
        (onChange || rebuildEditor)();
    }

    function renderBuilder() {
        els.builder.innerHTML = "";
        state.components.forEach(function (node, i) { els.builder.appendChild(renderNode(node, state.components, i)); });
        els.addBar.innerHTML = "";
        els.addBar.appendChild(makeAddBar(state.components, currentMode === "componentsv2" ? "v2top" : "normaltop"));
    }

    function defButton() { return { type: 2, style: 1, label: "Button", custom_id: "" }; }
    function defSelect() { return { type: 3, placeholder: "", custom_id: "", options: [{ label: "Option 1", value: "1" }] }; }
    function defActionRow() { return { type: 1, components: [] }; }
    function defText() { return { type: 10, content: "" }; }
    function defSection() { return { type: 9, components: [{ type: 10, content: "Section text" }], accessory: defButton() }; }
    function defContainer() { return { type: 17, accent_color: 5793266, components: [] }; }
    function defSeparator() { return { type: 14, divider: true, spacing: 1 }; }
    function defGallery() { return { type: 12, items: [{ media: { url: "" } }] }; }

    function makeAddBar(list, context) {
        let bar = el("div", { class: "mc-add-bar" });
        function add(label, factory) {
            bar.appendChild(btn(label, "btn-sm btn-outline-primary", function () { list.push(factory()); rebuildEditor(); }));
        }
        if (context === "normaltop") {
            add("+ Action row", defActionRow);
        } else {
            add("+ Text", defText);
            add("+ Action row", defActionRow);
            add("+ Section", defSection);
            if (context === "v2top") add("+ Container", defContainer);
            add("+ Separator", defSeparator);
            add("+ Media gallery", defGallery);
        }
        return bar;
    }

    function nodeHeader(title, list, idx, onChange) {
        onChange = onChange || rebuildEditor;
        let tools = el("div", { class: "mc-node-tools" }, [
            btn('<i class="fas fa-arrow-up"></i>', "btn-sm btn-secondary", function () { move(list, idx, -1, onChange); }, { title: "Move up" }),
            btn('<i class="fas fa-arrow-down"></i>', "btn-sm btn-secondary", function () { move(list, idx, 1, onChange); }, { title: "Move down" }),
            btn('<i class="fas fa-trash"></i>', "btn-sm btn-danger", function () { list.splice(idx, 1); onChange(); }, { noconfirm: true, title: "Remove" })
        ]);
        return el("div", { class: "mc-node-header" }, [el("span", { class: "mc-node-title" }, [title]), tools]);
    }

    function renderNode(node, list, idx) {
        let card = el("div", { class: "mc-node" });
        card.appendChild(nodeHeader(NODE_TITLES[node.type] || ("Type " + node.type), list, idx));
        let body;
        switch (node.type) {
            case 10: body = field("Text (markdown supported)", node.content, function (v) { node.content = v; refreshPreview(); }, { textarea: true, rows: 2 }); break;
            case 14: body = separatorBody(node); break;
            case 1: body = actionRowBody(node); break;
            case 9: body = sectionBody(node); break;
            case 17: body = containerBody(node); break;
            case 12: body = galleryBody(node); break;
            default: body = el("div", { class: "text-muted small" }, ["Unsupported component type " + node.type]);
        }
        card.appendChild(body);
        return card;
    }

    function separatorBody(node) {
        let wrap = el("div");
        let cb = el("input", { type: "checkbox" }); cb.className = "mr-2"; cb.checked = node.divider !== false;
        cb.addEventListener("change", function () { node.divider = cb.checked; refreshPreview(); });
        wrap.appendChild(el("label", { class: "small d-block mb-2" }, [cb, "Show divider line"]));
        wrap.appendChild(selectField("Spacing", [{ v: 1, t: "Small" }, { v: 2, t: "Large" }], node.spacing || 1, function (v) { node.spacing = parseInt(v); refreshPreview(); }));
        return wrap;
    }

    function actionRowBody(node) {
        node.components = node.components || [];
        let hasSelect = node.components.some(function (c) { return c.type !== 2; });
        let body = el("div");
        node.components.forEach(function (child, ci) { body.appendChild(interactiveEditor(child, node.components, ci)); });
        let bar = el("div", { class: "mc-add-bar mt-1" });
        if (node.components.length === 0) {
            bar.appendChild(btn("+ Button", "btn-block btn-outline-secondary", function () { node.components.push(defButton()); rebuildEditor(); }));
            bar.appendChild(btn("+ Select menu", "btn-block btn-outline-secondary", function () { node.components.push(defSelect()); rebuildEditor(); }));
            bar.appendChild(el("span", { class: "small text-muted ml-1" }, ["Up to 5 buttons, or one select menu."]));
        } else if (hasSelect) {
            bar.appendChild(el("span", { class: "small text-muted" }, ["A select menu takes up the whole row."]));
        } else if (node.components.length < 5) {
            bar.appendChild(btn("+ Button", "btn-block btn-outline-secondary", function () { node.components.push(defButton()); rebuildEditor(); }));
        }
        body.appendChild(bar);
        return body;
    }

    function interactiveEditor(child, list, idx) {
        let card = el("div", { class: "mc-node" });
        card.appendChild(nodeHeader(child.type === 2 ? "Button" : "Select menu", list, idx));
        card.appendChild(child.type === 2 ? buttonEditor(child) : selectEditor(child));
        return card;
    }

    function buttonEditor(node) {
        let wrap = el("div");
        wrap.appendChild(selectField("Style",
            [{ v: 1, t: "Primary" }, { v: 2, t: "Secondary" }, { v: 3, t: "Success" }, { v: 4, t: "Danger" }, { v: 5, t: "Link" }],
            node.style || 1, function (v) {
                node.style = parseInt(v);
                if (node.style === 5) { delete node.custom_id; if (node.url == null) node.url = ""; }
                else { delete node.url; if (node.custom_id == null) node.custom_id = ""; }
                rebuildEditor();
            }));
        wrap.appendChild(field("Label", node.label, function (v) { node.label = v; refreshPreview(); }));
        if (node.style === 5) {
            wrap.appendChild(field("URL", node.url, function (v) { node.url = v; refreshPreview(); }, { placeholder: "https://..." }));
        } else {
            wrap.appendChild(customIdField(node));
        }
        wrap.appendChild(field("Emoji (optional)", node.emoji && node.emoji.name, function (v) {
            if (v) node.emoji = { name: v }; else delete node.emoji; refreshPreview();
        }, { placeholder: "e.g. 🔥" }));
        return wrap;
    }

    function selectEditor(node) {
        let wrap = el("div");
        wrap.appendChild(selectField("Menu type",
            [{ v: 3, t: "Text options" }, { v: 5, t: "Users" }, { v: 6, t: "Roles" }, { v: 7, t: "Users & roles" }, { v: 8, t: "Channels" }],
            node.type || 3, function (v) {
                node.type = parseInt(v);
                if (node.type === 3) { if (!node.options) node.options = [{ label: "Option 1", value: "1" }]; }
                else { delete node.options; }
                rebuildEditor();
            }));
        wrap.appendChild(field("Placeholder", node.placeholder, function (v) { node.placeholder = v; refreshPreview(); }));
        wrap.appendChild(customIdField(node));
        wrap.appendChild(el("div", { class: "form-row" }, [
            el("div", { class: "col" }, [numField("Min values", node.min_values, function (v) { if (v == null) delete node.min_values; else node.min_values = v; refreshPreview(); })]),
            el("div", { class: "col" }, [numField("Max values", node.max_values, function (v) { if (v == null) delete node.max_values; else node.max_values = v; refreshPreview(); })])
        ]));
        // Only "Text options" menus carry an explicit option list; the other types are auto-populated by Discord.
        if (!node.type || node.type === 3) {
            node.options = node.options || [];
            wrap.appendChild(el("label", { class: "small mb-1 d-block" }, ["Options"]));
            node.options.forEach(function (opt, oi) {
                wrap.appendChild(el("div", { class: "mc-opt-row" }, [
                    inlineInput("Label", opt.label, function (v) { opt.label = v; refreshPreview(); }),
                    inlineInput("Value", opt.value, function (v) { opt.value = v; refreshPreview(); }),
                    inlineInput("Description", opt.description, function (v) { if (v) opt.description = v; else delete opt.description; refreshPreview(); }),
                    node.options.length > 1 ? btn('<i class="fas fa-times"></i>', "btn-block btn-danger", function () { node.options.splice(oi, 1); rebuildEditor(); }, { noconfirm: true }) : null
                ]));
            });
            if (node.options.length < 25) {
                wrap.appendChild(btn("+ Option", "btn-block btn-outline-secondary", function () {
                    node.options.push({ label: "Option " + (node.options.length + 1), value: String(node.options.length + 1) }); rebuildEditor();
                }));
            }
        }
        return wrap;
    }

    function sectionBody(node) {
        node.components = node.components || [defText()];
        node.accessory = node.accessory || defButton();
        let body = el("div");
        node.components.forEach(function (td, ti) {
            let ta = el("textarea", { class: "form-control form-control-sm", placeholder: "Text" }); ta.rows = 2; ta.value = td.content || "";
            ta.addEventListener("input", function () { td.content = ta.value; refreshPreview(); });
            body.appendChild(el("div", { class: "mc-opt-row" }, [
                ta,
                node.components.length > 1 ? btn('<i class="fas fa-times"></i>', "btn-block btn-danger", function () { node.components.splice(ti, 1); rebuildEditor(); }, { noconfirm: true }) : null
            ]));
        });
        if (node.components.length < 3) body.appendChild(btn("+ Text", "btn-block btn-outline-secondary", function () { node.components.push(defText()); rebuildEditor(); }));
        body.appendChild(el("hr", { class: "my-2" }));
        body.appendChild(selectField("Accessory", [{ v: 2, t: "Button" }, { v: 11, t: "Thumbnail" }], node.accessory.type, function (v) {
            node.accessory = parseInt(v) === 2 ? defButton() : { type: 11, media: { url: "" } };
            rebuildEditor();
        }));
        if (node.accessory.type === 2) body.appendChild(buttonEditor(node.accessory));
        else body.appendChild(field("Thumbnail URL", node.accessory.media && node.accessory.media.url, function (v) { node.accessory.media = { url: v }; refreshPreview(); }, { placeholder: "https://..." }));
        return body;
    }

    function containerBody(node) {
        node.components = node.components || [];
        let body = el("div");
        body.appendChild(colorField("Accent color", node.accent_color, function (v) {
            if (v == null) delete node.accent_color; else node.accent_color = v;
            refreshPreview();
        }));
        node.components.forEach(function (child, ci) { body.appendChild(renderNode(child, node.components, ci)); });
        body.appendChild(makeAddBar(node.components, "v2container"));
        return body;
    }

    function galleryBody(node) {
        node.items = node.items || [{ media: { url: "" } }];
        let body = el("div");
        node.items.forEach(function (it, ii) {
            it.media = it.media || { url: "" };
            body.appendChild(el("div", { class: "mc-opt-row" }, [
                inlineInput("Image / video URL", it.media.url, function (v) { it.media.url = v; refreshPreview(); }),
                inlineInput("Description (optional)", it.description, function (v) { if (v) it.description = v; else delete it.description; refreshPreview(); }),
                node.items.length > 1 ? btn('<i class="fas fa-times"></i>', "btn-block btn-danger", function () { node.items.splice(ii, 1); rebuildEditor(); }, { noconfirm: true }) : null
            ]));
        });
        if (node.items.length < 10) body.appendChild(btn("+ Item", "btn-block btn-outline-secondary", function () { node.items.push({ media: { url: "" } }); rebuildEditor(); }));
        return body;
    }


    // Finalize one interactive component's custom_id: link buttons carry none; locked components
    // (loaded without the templates- prefix) keep their id verbatim; everything else gets the
    // "templates-" prefix added to the user-entered suffix (blank stays blank for server auto-fill).
    function finalizeInteractive(c) {
        if (c.type === 2 && c.style === 5) { delete c.custom_id; delete c._locked; return; }
        if (c._locked) { delete c._locked; return; }
        delete c._locked;
        let s = (c.custom_id || "").trim();
        c.custom_id = s ? (TEMPLATE_PREFIX + s) : "";
    }

    function finalizeWalk(arr) {
        arr.forEach(function (c) {
            if (c.type === 1) (c.components || []).forEach(finalizeInteractive);
            else if (c.type === 17) finalizeWalk(c.components || []);
            else if (c.type === 9 && c.accessory) finalizeInteractive(c.accessory);
        });
    }

    // Loaded (edit) inverse of finalizeInteractive: strip the templates- prefix to show the bare
    // suffix; ids that were never templated are flagged _locked so the UI won't let them be edited.
    function adaptInteractive(c) {
        if (c.type === 2 && c.style === 5) return; // link button, no custom_id
        let id = c.custom_id || "";
        if (id.indexOf(TEMPLATE_PREFIX) === 0) { c.custom_id = id.slice(TEMPLATE_PREFIX.length); c._locked = false; }
        else if (id) { c._locked = true; }
        else { c.custom_id = ""; c._locked = false; }
    }

    function adaptLoadedComponents(arr) {
        arr.forEach(function (c) {
            if (c.type === 1) (c.components || []).forEach(adaptInteractive);
            else if (c.type === 17) adaptLoadedComponents(c.components || []);
            else if (c.type === 9 && c.accessory) adaptInteractive(c.accessory);
        });
    }

    // Drop empty action rows / containers so we never send an invalid (empty) row to Discord.
    function pruneEmpty(comps) {
        let out = [];
        comps.forEach(function (c) {
            if (c.type === 1) { if (c.components && c.components.length) out.push(c); }
            else if (c.type === 17) { c.components = pruneEmpty(c.components || []); if (c.components.length) out.push(c); }
            else out.push(c);
        });
        return out;
    }

    // Deep-clone the live model, finalize custom_ids, and prune empties for the outgoing payload.
    function buildComponents(comps) {
        let clone = JSON.parse(JSON.stringify(comps || []));
        finalizeWalk(clone);
        return pruneEmpty(clone);
    }

    function computeMessage() {
        let comps = buildComponents(state.components);
        if (currentMode === "componentsv2") return { message: { components: comps } };
        let msg = {};
        let content = fieldValue("embed-content"); if (content) msg.content = content;
        let embeds = buildEmbeds(); if (embeds.length) msg.embeds = embeds;
        if (comps.length) msg.components = comps;
        return { message: msg };
    }

    function renderPreview() {
        if (!els.preview) return;
        let msg = computeMessage().message;
        els.preview.innerHTML = "";
        if (msg.content) els.preview.appendChild(renderText(msg.content, "mc-content"));
        if (msg.embeds) msg.embeds.forEach(function (e) { els.preview.appendChild(renderEmbed(e)); });
        if (msg.components) renderComponents(msg.components, els.preview);
        if (!els.preview.childNodes.length) els.preview.appendChild(div("text-muted", "Nothing to preview yet."));
    }

    function renderEmbed(e) {
        let box = div("mc-embed");
        box.style.borderLeftColor = (typeof e.color === "number") ? intToHex(e.color) : "#4f545c";
        if (e.author && e.author.name) {
            let a = div("mc-embed-author");
            if (e.author.icon_url) a.appendChild(img(e.author.icon_url, "mc-embed-author-icon"));
            a.appendChild(span(e.author.name));
            box.appendChild(a);
        }
        if (e.title) box.appendChild(div("mc-embed-title", e.title)); // titles render no markdown on Discord
        if (e.description) { let d = div("mc-embed-desc"); d.appendChild(renderMarkdown(e.description)); box.appendChild(d); }
        if (e.fields && e.fields.length) {
            let grid = div("mc-embed-fields");
            e.fields.forEach(function (f) {
                let fd = div("mc-embed-field" + (f.inline ? " mc-inline" : ""));
                let fn = div("mc-embed-field-name"); fn.appendChild(renderInlineMarkdown(f.name || ""));
                let fv = div("mc-embed-field-value"); fv.appendChild(renderMarkdown(f.value || ""));
                fd.appendChild(fn); fd.appendChild(fv); grid.appendChild(fd);
            });
            box.appendChild(grid);
        }
        if (e.image && e.image.url) box.appendChild(img(e.image.url, "mc-embed-image"));
        if (e.thumbnail && e.thumbnail.url) box.appendChild(img(e.thumbnail.url, "mc-embed-thumb"));
        if (e.footer && (e.footer.text || e.timestamp)) {
            let f = div("mc-embed-footer");
            if (e.footer.icon_url) f.appendChild(img(e.footer.icon_url, "mc-embed-footer-icon"));
            let ftext = e.footer.text || "";
            if (e.timestamp) ftext += (ftext ? " • " : "") + new Date(e.timestamp).toLocaleString();
            f.appendChild(span(ftext));
            box.appendChild(f);
        }
        return box;
    }

    function renderComponents(comps, parent) { comps.forEach(function (c) { parent.appendChild(renderComponent(c)); }); }

    function renderComponent(c) {
        switch (c.type) {
            case 10: return renderText(c.content || "", "mc-v2-text");
            case 17:
                let ct = div("mc-v2-container");
                ct.style.borderLeftColor = (typeof c.accent_color === "number") ? intToHex(c.accent_color) : "transparent";
                renderComponents(c.components || [], ct);
                return ct;
            case 9:
                let sec = div("mc-v2-section");
                let body = div("mc-v2-section-body");
                renderComponents(c.components || [], body);
                sec.appendChild(body);
                if (c.accessory) sec.appendChild(renderAccessory(c.accessory));
                return sec;
            case 1:
                let row = div("mc-v2-row");
                (c.components || []).forEach(function (ic) { row.appendChild(renderInteractive(ic)); });
                return row;
            case 14:
                let sep = div("mc-v2-separator");
                if (c.divider !== false) sep.classList.add("mc-divider");
                return sep;
            case 12:
                let g = div("mc-v2-gallery");
                (c.items || []).forEach(function (it) { if (it.media && it.media.url) g.appendChild(img(it.media.url, "mc-v2-gallery-item")); });
                return g;
            default: return renderInteractive(c);
        }
    }

    function renderAccessory(acc) {
        if (acc.type === 11 && acc.media) return img(acc.media.url, "mc-v2-thumb");
        if (acc.type === 2) return renderInteractive(acc);
        return div("");
    }

    function renderInteractive(c) {
        switch (c.type) {
            case 2:
                let st = BUTTON_STYLES[c.style] || BUTTON_STYLES[2];
                let b = div("mc-v2-button", (c.emoji && c.emoji.name ? c.emoji.name + " " : "") + (c.label || (c.style === 5 ? "Link" : "Button")));
                b.style.backgroundColor = st.bg; b.style.color = st.fg;
                if (c.style === 5) { // link buttons show an external-link indicator, like Discord
                    let icon = document.createElement("i");
                    icon.className = "fas fa-external-link-alt mc-v2-link-icon";
                    b.appendChild(icon);
                }
                return b;
            case 3: case 5: case 6: case 7: case 8:
                return div("mc-v2-select", c.placeholder || "Select…");
            default:
                return div("mc-v2-unknown", "[component type " + c.type + "]");
        }
    }

    function renderText(text, cls) { let d = div(cls || ""); d.appendChild(renderMarkdown(text)); return d; }


    function loadMessage() {
        let link = document.getElementById("mc-edit-link").value.trim();
        let errEl = document.getElementById("mc-load-error");
        errEl.className = "text-small mt-1 text-danger";
        errEl.textContent = "";
        if (!link) { errEl.textContent = "Paste a message link first."; return; }

        fetch(base() + "/load?link=" + encodeURIComponent(link), { credentials: "same-origin" })
            .then(function (r) { return r.json().then(function (j) { return { ok: r.ok, body: j }; }); })
            .then(function (res) {
                if (!res.ok || res.body.error) { errEl.textContent = res.body.error || "Failed to load message."; return; }
                populateFromMessage(res.body);
                if (!res.body.author_is_bot) {
                    errEl.className = "text-small mt-1 text-danger";
                    errEl.textContent = "This message was not sent by YAGPDB, so it can't be edited (switch to Create to send it as a new message).";
                    loadedForEdit = false; els.submit.disabled = true; show(els.editHint, true);
                } else {
                    loadedForEdit = true; els.submit.disabled = false; show(els.editHint, false);
                }
            })
            .catch(function (e) { errEl.textContent = "Failed to load message: " + e.message; });
    }

    function populateFromMessage(resp) {
        let payload = resp.payload || {};
        let mode = resp.mode === "componentsv2" ? "componentsv2" : "normal";
        state.components = (payload.components && payload.components.length) ? payload.components : [];
        adaptLoadedComponents(state.components);
        setMode(mode);

        if (mode === "normal") {
            setVal("embed-content", payload.content || "");
            state.embeds = (payload.embeds || []).map(function (e) {
                return {
                    title: e.title || "", url: e.url || "", description: e.description || "",
                    color: typeof e.color === "number" ? e.color : undefined,
                    authorName: (e.author && e.author.name) || "", authorIcon: (e.author && e.author.icon_url) || "",
                    footerText: (e.footer && e.footer.text) || "", footerIcon: (e.footer && e.footer.icon_url) || "",
                    imageUrl: (e.image && e.image.url) || "", thumbUrl: (e.thumbnail && e.thumbnail.url) || "",
                    ts: !!e.timestamp,
                    fields: (e.fields || []).map(function (f) { return { name: f.name || "", value: f.value || "", inline: !!f.inline }; })
                };
            });
        } else {
            state.embeds = [];
        }

        if (currentAction === "edit") setTypeLock(mode);
        renderEmbeds();
        rebuildEditor();
    }


    function el(tag, attrs, children) {
        let e = document.createElement(tag);
        if (attrs) Object.keys(attrs).forEach(function (k) {
            if (k === "class") e.className = attrs[k];
            else e.setAttribute(k, attrs[k]);
        });
        (children || []).forEach(function (c) {
            if (c == null) return;
            e.appendChild(typeof c === "string" ? document.createTextNode(c) : c);
        });
        return e;
    }

    function btn(html, cls, onclick, opts) {
        opts = opts || {};
        let b = document.createElement("button");
        b.type = "button"; b.className = "btn " + cls; b.innerHTML = html;
        if (opts.noconfirm) b.setAttribute("noconfirm", "true");
        if (opts.title) b.title = opts.title;
        b.addEventListener("click", onclick);
        return b;
    }

    function field(labelText, value, oninput, opts) {
        opts = opts || {};
        let input = document.createElement(opts.textarea ? "textarea" : "input");
        input.className = "form-control form-control-sm";
        if (opts.textarea) input.rows = opts.rows || 2;
        if (opts.placeholder) input.placeholder = opts.placeholder;
        if (opts.maxlength) input.maxLength = opts.maxlength;
        input.value = value == null ? "" : value;
        input.addEventListener("input", function () { oninput(input.value); });
        let children = [el("label", { class: "small mb-1" }, [labelText]), input];
        if (opts.help) children.push(el("small", { class: "form-text text-muted" }, [opts.help]));
        return el("div", { class: "form-group mb-2" }, children);
    }

    function readonlyField(labelText, value, help) {
        let input = el("input", { class: "form-control form-control-sm" }); input.value = value || ""; input.disabled = true;
        let children = [el("label", { class: "small mb-1" }, [labelText]), input];
        if (help) children.push(el("small", { class: "form-text text-muted" }, [help]));
        return el("div", { class: "form-group mb-2" }, children);
    }

    // The editable custom_id field holds the bare suffix; the "templates-" prefix is added on build.
    // Components loaded without that prefix are "locked" — shown read-only so their id is preserved.
    function customIdField(node) {
        if (node._locked) {
            return readonlyField("Custom ID (fixed)", node.custom_id,
                "This component wasn't created with the templates- prefix, so its id can't be changed.");
        }
        return field("Custom ID (optional)", node.custom_id, function (v) { node.custom_id = v; refreshPreview(); },
            { placeholder: "auto-generated", maxlength: MAX_SUFFIX, help: '"templates-" is added automatically so it can trigger a custom command.' });
    }

    function inlineInput(placeholder, value, oninput) {
        let i = document.createElement("input");
        i.className = "form-control form-control-sm"; i.placeholder = placeholder;
        i.value = value == null ? "" : value;
        i.addEventListener("input", function () { oninput(i.value); });
        return i;
    }

    function numField(labelText, value, oninput) {
        let i = document.createElement("input");
        i.type = "number"; i.className = "form-control form-control-sm"; i.min = 0;
        i.value = value == null ? "" : value;
        i.addEventListener("input", function () { oninput(i.value === "" ? null : parseInt(i.value)); });
        return el("div", { class: "form-group mb-2" }, [el("label", { class: "small mb-1" }, [labelText]), i]);
    }

    function selectField(labelText, options, value, onchange) {
        let sel = document.createElement("select");
        sel.className = "form-control form-control-sm";
        options.forEach(function (o) {
            let op = document.createElement("option");
            op.value = String(o.v); op.textContent = o.t;
            if (String(o.v) === String(value)) op.selected = true;
            sel.appendChild(op);
        });
        sel.addEventListener("change", function () { onchange(sel.value); });
        return el("div", { class: "form-group mb-2" }, [el("label", { class: "small mb-1" }, [labelText]), sel]);
    }

    function show(elm, on) { if (elm) elm.style.display = on ? "" : "none"; }
    function setVal(id, v) { let e = document.getElementById(id); if (e) e.value = v; }
    function setChecked(id, v) { let e = document.getElementById(id); if (e) e.checked = !!v; }
    function hexToInt(hex) { return hex ? (parseInt(hex.replace("#", ""), 16) || 0) : 0; }
    function intToHex(i) { let s = (i & 0xffffff).toString(16); while (s.length < 6) s = "0" + s; return "#" + s; }

    function div(cls, text) { let d = document.createElement("div"); if (cls) d.className = cls; if (text != null) d.textContent = text; return d; }
    function span(text) { let s = document.createElement("span"); s.textContent = text; return s; }
    function img(url, cls) {
        let i = document.createElement("img");
        i.src = url; if (cls) i.className = cls;
        i.onerror = function () { i.style.display = "none"; };
        return i;
    }

    // ---- Markdown renderer ----
    let INLINE_RULES = [
        { regex: /\|\|([\s\S]+?)\|\|/, tag: "span", className: "mc-md-spoiler", recurse: true },
        { regex: /\*\*\*([\s\S]+?)\*\*\*/, build: buildBoldItalicNode },
        { regex: /\*\*([\s\S]+?)\*\*/, tag: "strong", recurse: true },
        { regex: /__([\s\S]+?)__/, tag: "u", recurse: true },
        { regex: /\*([^\s*][\s\S]*?)\*/, tag: "em", recurse: true },
        { regex: /_([^_]+?)_/, tag: "em", recurse: true },
        { regex: /~~([\s\S]+?)~~/, tag: "s", recurse: true },
        { regex: /`([^`\n]+)`/, tag: "code", className: "mc-md-code", recurse: false },
        { regex: /\[([^\]]+)\]\((https?:\/\/[^\s)]+)\)/, build: buildLinkNode }
    ];

    function buildBoldItalicNode(match) {
        let strong = document.createElement("strong");
        let em = document.createElement("em");
        appendInlineMarkdown(em, match[1]);
        strong.appendChild(em);
        return strong;
    }

    function buildLinkNode(match) {
        if (!/^https?:\/\//i.test(match[2])) return document.createTextNode(match[0]);
        let anchor = document.createElement("a");
        anchor.href = match[2];
        anchor.target = "_blank";
        anchor.rel = "noopener noreferrer";
        appendInlineMarkdown(anchor, match[1]);
        return anchor;
    }

    function buildInlineNode(rule, match) {
        if (rule.build) return rule.build(match);
        let node = document.createElement(rule.tag);
        if (rule.className) node.className = rule.className;
        if (rule.recurse) appendInlineMarkdown(node, match[1]);
        else node.textContent = match[1]; // inline code: contents are literal
        return node;
    }

    // Parses inline markdown in `text`, appending the resulting nodes to `parent`.
    function appendInlineMarkdown(parent, text) {
        while (text.length) {
            let earliest = null;
            for (let i = 0; i < INLINE_RULES.length; i++) {
                let match = INLINE_RULES[i].regex.exec(text);
                if (match && (!earliest || match.index < earliest.match.index)) {
                    earliest = { rule: INLINE_RULES[i], match: match };
                }
            }
            if (!earliest) { parent.appendChild(document.createTextNode(text)); return; }
            if (earliest.match.index > 0) parent.appendChild(document.createTextNode(text.slice(0, earliest.match.index)));
            parent.appendChild(buildInlineNode(earliest.rule, earliest.match));
            text = text.slice(earliest.match.index + earliest.match[0].length);
        }
    }

    // Inline-only markdown (no headings / lists / code blocks), e.g. for embed field names.
    function renderInlineMarkdown(text) {
        let fragment = document.createDocumentFragment();
        appendInlineMarkdown(fragment, String(text));
        return fragment;
    }

    let BLOCK_PATTERNS = {
        bullet: /^\s*[-*]\s+(.*)$/,
        ordered: /^\s*\d+\.\s+(.*)$/,
        heading: /^(#{1,3})\s+(.*)$/,
        subtext: /^-#\s+(.*)$/,
        quote: /^>\s?(.*)$/
    };

    function isBlockLine(line) {
        return BLOCK_PATTERNS.bullet.test(line) || BLOCK_PATTERNS.ordered.test(line) ||
            BLOCK_PATTERNS.heading.test(line) || BLOCK_PATTERNS.subtext.test(line) || BLOCK_PATTERNS.quote.test(line);
    }

    // Full Discord markdown (code blocks, headings, subtext, quotes, lists, inline marks), e.g. for
    // message content, Text Display components, embed descriptions and field values.
    function renderMarkdown(text) {
        let fragment = document.createDocumentFragment();
        // Split on fenced code blocks; odd indices hold the (literal) code body.
        let segments = String(text).split(/```(?:[a-zA-Z0-9_+\-]*\n)?([\s\S]*?)```/);
        segments.forEach(function (segment, index) {
            if (index % 2 === 1) fragment.appendChild(buildCodeBlockNode(segment.replace(/\n$/, "")));
            else if (segment) appendMarkdownBlocks(fragment, segment);
        });
        return fragment;
    }

    function buildCodeBlockNode(code) {
        let pre = document.createElement("pre");
        pre.className = "mc-md-codeblock";
        let codeEl = document.createElement("code");
        codeEl.textContent = code;
        pre.appendChild(codeEl);
        return pre;
    }

    function buildBlockNode(className, text) {
        let node = document.createElement("div");
        node.className = className;
        appendInlineMarkdown(node, text);
        return node;
    }

    function appendMarkdownBlocks(parent, text) {
        let lines = text.split("\n");
        let i = 0;
        while (i < lines.length) {
            let line = lines[i];
            if (BLOCK_PATTERNS.bullet.test(line) || BLOCK_PATTERNS.ordered.test(line)) {
                i = appendListBlock(parent, lines, i, BLOCK_PATTERNS.ordered.test(line));
                continue;
            }
            let heading = BLOCK_PATTERNS.heading.exec(line);
            if (heading) { parent.appendChild(buildBlockNode("mc-md-h mc-md-h" + heading[1].length, heading[2])); i++; continue; }
            let subtext = BLOCK_PATTERNS.subtext.exec(line);
            if (subtext) { parent.appendChild(buildBlockNode("mc-md-subtext", subtext[1])); i++; continue; }
            let quote = BLOCK_PATTERNS.quote.exec(line);
            if (quote) {
                let blockquote = document.createElement("blockquote");
                blockquote.className = "mc-md-quote";
                appendInlineMarkdown(blockquote, quote[1]);
                parent.appendChild(blockquote);
                i++;
                continue;
            }
            i = appendParagraphBlock(parent, lines, i);
        }
    }

    // Appends consecutive list items of one kind; returns the index of the next unconsumed line.
    function appendListBlock(parent, lines, start, ordered) {
        let listEl = document.createElement(ordered ? "ol" : "ul");
        listEl.className = "mc-md-list";
        let pattern = ordered ? BLOCK_PATTERNS.ordered : BLOCK_PATTERNS.bullet;
        let i = start, match;
        for (; i < lines.length && (match = pattern.exec(lines[i])); i++) {
            let item = document.createElement("li");
            appendInlineMarkdown(item, match[1]);
            listEl.appendChild(item);
        }
        parent.appendChild(listEl);
        return i;
    }

    // Appends a run of consecutive non-block lines as one paragraph (lines joined by <br>); returns
    // the index of the next unconsumed line.
    function appendParagraphBlock(parent, lines, start) {
        let paragraph = document.createElement("div");
        paragraph.className = "mc-md-line";
        let i = start;
        for (; i < lines.length && !isBlockLine(lines[i]); i++) {
            if (i > start) paragraph.appendChild(document.createElement("br"));
            appendInlineMarkdown(paragraph, lines[i]);
        }
        parent.appendChild(paragraph);
        return i;
    }

    init();
})();
