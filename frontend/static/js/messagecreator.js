// NOTE: the control panel loads pages via partial AJAX navigation ($("#main-content").html(...)),
// so DOMContentLoaded does NOT fire on navigation. We initialise immediately and scope every
// listener to #mc-root. Toggles are driven by explicit label clicks (Bootstrap's
// data-toggle="buttons" fires a jQuery-only change event native listeners never receive).
(function () {
    "use strict";

    var BUTTON_STYLES = {
        1: { bg: "#5865f2", fg: "#fff" },
        2: { bg: "#4f545c", fg: "#fff" },
        3: { bg: "#248046", fg: "#fff" },
        4: { bg: "#da373c", fg: "#fff" },
        5: { bg: "#4f545c", fg: "#fff" }
    };

    var NODE_TITLES = { 1: "Action Row", 9: "Section", 10: "Text", 12: "Media Gallery", 14: "Separator", 17: "Container" };

    var TEMPLATE_PREFIX = "templates-";
    var MAX_CUSTOM_ID = 100;                                 // Discord limit, including the prefix
    var MAX_SUFFIX = MAX_CUSTOM_ID - TEMPLATE_PREFIX.length; // editable part the user types
    var PRESET_COLORS = ["#5865f2", "#57f287", "#fee75c", "#eb459e", "#ed4245", "#1abc9c",
        "#3498db", "#9b59b6", "#e67e22", "#f1c40f", "#95a5a6", "#ffffff", "#2c2f33", "#000000"];

    var currentMode = "normal";
    var currentAction = "create";
    var loadedForEdit = false;
    var state = { embeds: [], components: [] };
    var els = {};

    function init() {
        var root = document.getElementById("mc-root");
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
                var input = label.querySelector("input");
                if (input) setAction(input.value);
            });
        });
        root.querySelectorAll(".mc-mode-toggle label").forEach(function (label) {
            label.addEventListener("click", function (e) {
                e.preventDefault();
                if (label.classList.contains("disabled")) return;
                var input = label.querySelector("input");
                if (input) setMode(input.value);
            });
        });

        root.addEventListener("input", function (e) {
            if (e.target.classList && (e.target.classList.contains("mc-input") || e.target.classList.contains("mc-field-input"))) {
                renderPreview();
            }
        });

        var loadBtn = document.getElementById("mc-load-btn");
        if (loadBtn) loadBtn.addEventListener("click", loadMessage);

        els.form.addEventListener("submit", onSubmit);

        setAction("create");
        setMode("normal");
        renderEmbeds();
    }

    function base() { return "/manage/" + els.guildID + "/messagecreator"; }

    function syncToggle(name) {
        els.root.querySelectorAll("input[name=" + name + "]").forEach(function (r) {
            var label = r.closest("label");
            if (label) label.classList.toggle("active", r.checked);
        });
    }

    function setAction(action) {
        currentAction = action;
        var creating = action === "create";
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

        var v2 = mode === "componentsv2";
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
        var radios = els.root.querySelectorAll("input[name=mc-mode]");
        if (lockedMode) {
            setMode(lockedMode);
            radios.forEach(function (r) {
                var label = r.closest("label");
                if (label) label.classList.toggle("disabled", r.value !== lockedMode);
            });
            els.typeLockNote.style.display = "";
            els.typeLockNote.textContent = lockedMode === "componentsv2"
                ? "This is a Components V2 message — its type can't be changed when editing."
                : "Editing an existing message — its type can't be changed.";
        } else {
            radios.forEach(function (r) {
                var label = r.closest("label");
                if (label) label.classList.remove("disabled");
            });
            els.typeLockNote.style.display = "none";
        }
    }

    function onSubmit(e) {
        els.form.action = base() + "/" + (currentAction === "create" ? "send" : "edit");
        var msg = computeMessage().message;
        var errs = validateComposed(msg, currentMode);
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

    function rc(s) { return s ? String(s).length : 0; }
    function isURL(s) { return /^https?:\/\//i.test(s) || /^attachment:\/\//i.test(s); }

    function validateComposed(msg, mode) {
        var errs = [];
        if (mode === "normal") {
            if (rc(msg.content) > 2000) errs.push("Message content exceeds 2000 characters.");
            var embeds = msg.embeds || [];
            if (embeds.length > 10) errs.push("A message can have at most 10 embeds.");
            embeds.forEach(function (e, i) { validateEmbed(e, i + 1, errs); });
            if (!msg.content && !embeds.length && !(msg.components || []).length) {
                errs.push("Add content, an embed, or a component before sending.");
            }
            var rows = (msg.components || []).filter(function (c) { return c.type === 1; }).length;
            if (rows > 5) errs.push("A message can have at most 5 action rows.");
        } else {
            if (!(msg.components || []).length) errs.push("Add at least one component for a Components V2 message.");
            if ((msg.components || []).length > 10) errs.push("A Components V2 message can have at most 10 top-level components.");
        }
        validateComponents(msg.components || [], errs);
        return errs;
    }

    function validateEmbed(e, n, errs) {
        var total = 0;
        if (rc(e.title) > 256) errs.push("Embed " + n + ": title exceeds 256 characters.");
        if (rc(e.description) > 4096) errs.push("Embed " + n + ": description exceeds 4096 characters.");
        total += rc(e.title) + rc(e.description);
        if (e.author) { if (rc(e.author.name) > 256) errs.push("Embed " + n + ": author name exceeds 256 characters."); total += rc(e.author.name); }
        if (e.footer) { if (rc(e.footer.text) > 2048) errs.push("Embed " + n + ": footer text exceeds 2048 characters."); total += rc(e.footer.text); }
        var fields = e.fields || [];
        if (fields.length > 25) errs.push("Embed " + n + ": at most 25 fields.");
        fields.forEach(function (f, fi) {
            if (!f.name || !f.value) errs.push("Embed " + n + " field " + (fi + 1) + ": both name and value are required.");
            if (rc(f.name) > 256) errs.push("Embed " + n + " field " + (fi + 1) + ": name exceeds 256 characters.");
            if (rc(f.value) > 1024) errs.push("Embed " + n + " field " + (fi + 1) + ": value exceeds 1024 characters.");
            total += rc(f.name) + rc(f.value);
        });
        if (total > 6000) errs.push("Embed " + n + ": total text exceeds 6000 characters.");
        [e.url, e.image && e.image.url, e.thumbnail && e.thumbnail.url, e.author && e.author.icon_url, e.footer && e.footer.icon_url]
            .forEach(function (u) { if (u && !isURL(u)) errs.push("Embed " + n + ": \"" + u + "\" is not a valid URL (must start with http:// or https://)."); });
    }

    function validateComponents(comps, errs) {
        comps.forEach(function (c) {
            switch (c.type) {
                case 1:
                    var items = c.components || [];
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
                    if (rc(c.content) > 4000) errs.push("A text component exceeds 4000 characters.");
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
            if (rc(c.label) > 80) errs.push("A button label exceeds 80 characters.");
            if (c.custom_id && rc(c.custom_id) > MAX_CUSTOM_ID) errs.push("A button custom id is too long (max " + MAX_SUFFIX + " characters, plus the templates- prefix).");
        } else {
            if (rc(c.placeholder) > 150) errs.push("A select placeholder exceeds 150 characters.");
            if (c.custom_id && rc(c.custom_id) > MAX_CUSTOM_ID) errs.push("A select custom id is too long (max " + MAX_SUFFIX + " characters, plus the templates- prefix).");
            if (!c.type || c.type === 3) {
                var opts = c.options || [];
                if (opts.length < 1 || opts.length > 25) errs.push("A text-options select menu needs between 1 and 25 options.");
                var seen = {};
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


    function val(id) { var el = document.getElementById(id); return el ? el.value.trim() : ""; }

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
        var card = el("div", { class: "mc-node" });
        card.appendChild(nodeHeader("Embed " + (idx + 1), state.embeds, idx, rebuildEmbeds));

        card.appendChild(el("div", { class: "form-row" }, [
            el("div", { class: "col-md-8" }, [field("Title", e.title, function (v) { e.title = v; touch(); })]),
            el("div", { class: "col-md-4" }, [colorField("Color", e.color, function (v) { e.color = v; touch(); })])
        ]));
        card.appendChild(field("Title URL", e.url, function (v) { e.url = v; touch(); }, { placeholder: "https://..." }));
        card.appendChild(field("Description", e.description, function (v) { e.description = v; touch(); }, { textarea: true, rows: 3 }));
        card.appendChild(el("div", { class: "form-row" }, [
            el("div", { class: "col-md-6" }, [field("Image URL", e.imageUrl, function (v) { e.imageUrl = v; touch(); }, { placeholder: "https://..." })]),
            el("div", { class: "col-md-6" }, [field("Thumbnail URL", e.thumbUrl, function (v) { e.thumbUrl = v; touch(); }, { placeholder: "https://..." })])
        ]));
        card.appendChild(el("div", { class: "form-row" }, [
            el("div", { class: "col-md-6" }, [field("Author name", e.authorName, function (v) { e.authorName = v; touch(); })]),
            el("div", { class: "col-md-6" }, [field("Author icon URL", e.authorIcon, function (v) { e.authorIcon = v; touch(); }, { placeholder: "https://..." })])
        ]));
        card.appendChild(el("div", { class: "form-row" }, [
            el("div", { class: "col-md-8" }, [field("Footer text", e.footerText, function (v) { e.footerText = v; touch(); })]),
            el("div", { class: "col-md-4" }, [field("Footer icon URL", e.footerIcon, function (v) { e.footerIcon = v; touch(); }, { placeholder: "https://..." })])
        ]));
        var tsCb = el("input", { type: "checkbox" }); tsCb.className = "mr-2"; tsCb.checked = !!e.ts;
        tsCb.addEventListener("change", function () { e.ts = tsCb.checked; touch(); });
        card.appendChild(el("label", { class: "d-flex align-items-center mb-2" }, [tsCb, "Include current timestamp in footer"]));

        card.appendChild(el("hr", { class: "my-2" }));
        card.appendChild(el("label", { class: "small d-block mb-1" }, ["Fields"]));
        e.fields = e.fields || [];
        e.fields.forEach(function (f, fi) {
            var inlineCb = el("input", { type: "checkbox" }); inlineCb.checked = !!f.inline;
            inlineCb.addEventListener("change", function () { f.inline = inlineCb.checked; touch(); });
            card.appendChild(el("div", { class: "mc-opt-row" }, [
                inlineInput("Field name", f.name, function (v) { f.name = v; touch(); }),
                inlineInput("Field value", f.value, function (v) { f.value = v; touch(); }),
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
        var out = [];
        state.embeds.forEach(function (e) {
            var emb = {};
            if (e.title) emb.title = e.title;
            if (e.url) emb.url = e.url;
            if (e.description) emb.description = e.description;
            if (typeof e.color === "number") emb.color = e.color;
            if (e.authorName || e.authorIcon) { emb.author = {}; if (e.authorName) emb.author.name = e.authorName; if (e.authorIcon) emb.author.icon_url = e.authorIcon; }
            if (e.footerText || e.footerIcon) { emb.footer = {}; if (e.footerText) emb.footer.text = e.footerText; if (e.footerIcon) emb.footer.icon_url = e.footerIcon; }
            if (e.imageUrl) emb.image = { url: e.imageUrl };
            if (e.thumbUrl) emb.thumbnail = { url: e.thumbUrl };
            if (e.ts) emb.timestamp = new Date().toISOString();
            var fields = (e.fields || []).filter(function (f) { return f.name || f.value; })
                .map(function (f) { return { name: f.name || "", value: f.value || "", inline: !!f.inline }; });
            if (fields.length) emb.fields = fields;
            if (Object.keys(emb).length) out.push(emb);
        });
        return out;
    }

    // A checkbox + color picker bound to an optional numeric color (undefined = no color).
    function colorPicker(value, oninput) {
        var cur = (typeof value === "number") ? intToHex(value) : "#5865f2";
        var swatch = el("span", { class: "mc-color-swatch" }); swatch.style.background = cur;
        var hexIn = el("input", { class: "form-control form-control-sm", maxlength: "7" });
        hexIn.value = cur; hexIn.style.maxWidth = "110px";
        function apply(h, fromHexInput) {
            h = h.trim(); if (h[0] !== "#") h = "#" + h;
            if (!/^#[0-9a-fA-F]{6}$/.test(h)) return;
            swatch.style.background = h;
            if (!fromHexInput) hexIn.value = h;
            oninput(hexToInt(h));
        }
        hexIn.addEventListener("input", function () { apply(hexIn.value, true); });
        var palette = el("div", { class: "mc-color-palette" });
        PRESET_COLORS.forEach(function (c) {
            var s = el("span", { class: "mc-color-swatch mc-color-preset" });
            s.style.background = c; s.title = c;
            s.addEventListener("click", function () { apply(c, false); });
            palette.appendChild(s);
        });
        return el("div", {}, [el("div", { class: "d-flex align-items-center", style: "gap:6px;" }, [swatch, hexIn]), palette]);
    }

    // Optional-color field: a checkbox enables the color picker; unchecked means no color.
    function colorField(labelText, value, oninput) {
        var selected = (typeof value === "number") ? value : hexToInt("#5865f2");
        var enabled = typeof value === "number";
        var cb = el("input", { type: "checkbox" }); cb.className = "mr-2"; cb.checked = enabled;
        var pickerWrap = el("div", { class: "mt-1" }, [colorPicker(value, function (v) { selected = v; if (cb.checked) oninput(v); })]);
        show(pickerWrap, enabled);
        cb.addEventListener("change", function () { show(pickerWrap, cb.checked); oninput(cb.checked ? selected : undefined); });
        return el("div", { class: "form-group mb-2" }, [
            el("label", { class: "small mb-1 d-block" }, [cb, labelText]),
            pickerWrap
        ]);
    }

    function showCompError(msg) { if (els.compError) els.compError.textContent = msg || ""; }


    // touch: a field changed — refresh the preview without rebuilding the editor (keeps input
    // focus). rebuild: structure changed — re-render the whole builder.
    function touch() { renderPreview(); }
    function rebuild() { renderBuilder(); renderPreview(); }

    function rebuildEmbeds() { renderEmbeds(); renderPreview(); }

    function move(list, idx, delta, onChange) {
        var j = idx + delta;
        if (j < 0 || j >= list.length) return;
        var tmp = list[idx]; list[idx] = list[j]; list[j] = tmp;
        (onChange || rebuild)();
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
        var bar = el("div", { class: "mc-add-bar" });
        function add(label, factory) {
            bar.appendChild(btn(label, "btn-sm btn-outline-primary", function () { list.push(factory()); rebuild(); }));
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
        onChange = onChange || rebuild;
        var tools = el("div", { class: "mc-node-tools" }, [
            btn('<i class="fas fa-arrow-up"></i>', "btn-sm btn-secondary", function () { move(list, idx, -1, onChange); }, { title: "Move up" }),
            btn('<i class="fas fa-arrow-down"></i>', "btn-sm btn-secondary", function () { move(list, idx, 1, onChange); }, { title: "Move down" }),
            btn('<i class="fas fa-trash"></i>', "btn-sm btn-danger", function () { list.splice(idx, 1); onChange(); }, { noconfirm: true, title: "Remove" })
        ]);
        return el("div", { class: "mc-node-header" }, [el("span", { class: "mc-node-title" }, [title]), tools]);
    }

    function renderNode(node, list, idx) {
        var card = el("div", { class: "mc-node" });
        card.appendChild(nodeHeader(NODE_TITLES[node.type] || ("Type " + node.type), list, idx));
        var body;
        switch (node.type) {
            case 10: body = field("Text (markdown supported)", node.content, function (v) { node.content = v; touch(); }, { textarea: true, rows: 2 }); break;
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
        var wrap = el("div");
        var cb = el("input", { type: "checkbox" }); cb.className = "mr-2"; cb.checked = node.divider !== false;
        cb.addEventListener("change", function () { node.divider = cb.checked; touch(); });
        wrap.appendChild(el("label", { class: "small d-block mb-2" }, [cb, "Show divider line"]));
        wrap.appendChild(selectField("Spacing", [{ v: 1, t: "Small" }, { v: 2, t: "Large" }], node.spacing || 1, function (v) { node.spacing = parseInt(v); touch(); }));
        return wrap;
    }

    function actionRowBody(node) {
        node.components = node.components || [];
        var hasSelect = node.components.some(function (c) { return c.type !== 2; });
        var body = el("div");
        node.components.forEach(function (child, ci) { body.appendChild(interactiveEditor(child, node.components, ci)); });
        var bar = el("div", { class: "mc-add-bar mt-1" });
        if (node.components.length === 0) {
            bar.appendChild(btn("+ Button", "btn-block btn-outline-secondary", function () { node.components.push(defButton()); rebuild(); }));
            bar.appendChild(btn("+ Select menu", "btn-block btn-outline-secondary", function () { node.components.push(defSelect()); rebuild(); }));
            bar.appendChild(el("span", { class: "small text-muted ml-1" }, ["Up to 5 buttons, or one select menu."]));
        } else if (hasSelect) {
            bar.appendChild(el("span", { class: "small text-muted" }, ["A select menu takes up the whole row."]));
        } else if (node.components.length < 5) {
            bar.appendChild(btn("+ Button", "btn-block btn-outline-secondary", function () { node.components.push(defButton()); rebuild(); }));
        }
        body.appendChild(bar);
        return body;
    }

    function interactiveEditor(child, list, idx) {
        var card = el("div", { class: "mc-node" });
        card.appendChild(nodeHeader(child.type === 2 ? "Button" : "Select menu", list, idx));
        card.appendChild(child.type === 2 ? buttonEditor(child) : selectEditor(child));
        return card;
    }

    function buttonEditor(node) {
        var wrap = el("div");
        wrap.appendChild(selectField("Style",
            [{ v: 1, t: "Primary" }, { v: 2, t: "Secondary" }, { v: 3, t: "Success" }, { v: 4, t: "Danger" }, { v: 5, t: "Link" }],
            node.style || 1, function (v) {
                node.style = parseInt(v);
                if (node.style === 5) { delete node.custom_id; if (node.url == null) node.url = ""; }
                else { delete node.url; if (node.custom_id == null) node.custom_id = ""; }
                rebuild();
            }));
        wrap.appendChild(field("Label", node.label, function (v) { node.label = v; touch(); }));
        if (node.style === 5) {
            wrap.appendChild(field("URL", node.url, function (v) { node.url = v; touch(); }, { placeholder: "https://..." }));
        } else {
            wrap.appendChild(customIdField(node));
        }
        wrap.appendChild(field("Emoji (optional)", node.emoji && node.emoji.name, function (v) {
            if (v) node.emoji = { name: v }; else delete node.emoji; touch();
        }, { placeholder: "e.g. 🔥" }));
        return wrap;
    }

    function selectEditor(node) {
        var wrap = el("div");
        wrap.appendChild(selectField("Menu type",
            [{ v: 3, t: "Text options" }, { v: 5, t: "Users" }, { v: 6, t: "Roles" }, { v: 7, t: "Users & roles" }, { v: 8, t: "Channels" }],
            node.type || 3, function (v) {
                node.type = parseInt(v);
                if (node.type === 3) { if (!node.options) node.options = [{ label: "Option 1", value: "1" }]; }
                else { delete node.options; }
                rebuild();
            }));
        wrap.appendChild(field("Placeholder", node.placeholder, function (v) { node.placeholder = v; touch(); }));
        wrap.appendChild(customIdField(node));
        wrap.appendChild(el("div", { class: "form-row" }, [
            el("div", { class: "col" }, [numField("Min values", node.min_values, function (v) { if (v == null) delete node.min_values; else node.min_values = v; touch(); })]),
            el("div", { class: "col" }, [numField("Max values", node.max_values, function (v) { if (v == null) delete node.max_values; else node.max_values = v; touch(); })])
        ]));
        // Only "Text options" menus carry an explicit option list; the other types are auto-populated by Discord.
        if (!node.type || node.type === 3) {
            node.options = node.options || [];
            wrap.appendChild(el("label", { class: "small mb-1 d-block" }, ["Options"]));
            node.options.forEach(function (opt, oi) {
                wrap.appendChild(el("div", { class: "mc-opt-row" }, [
                    inlineInput("Label", opt.label, function (v) { opt.label = v; touch(); }),
                    inlineInput("Value", opt.value, function (v) { opt.value = v; touch(); }),
                    inlineInput("Description", opt.description, function (v) { if (v) opt.description = v; else delete opt.description; touch(); }),
                    node.options.length > 1 ? btn('<i class="fas fa-times"></i>', "btn-block btn-danger", function () { node.options.splice(oi, 1); rebuild(); }, { noconfirm: true }) : null
                ]));
            });
            if (node.options.length < 25) {
                wrap.appendChild(btn("+ Option", "btn-block btn-outline-secondary", function () {
                    node.options.push({ label: "Option " + (node.options.length + 1), value: String(node.options.length + 1) }); rebuild();
                }));
            }
        }
        return wrap;
    }

    function sectionBody(node) {
        node.components = node.components || [defText()];
        node.accessory = node.accessory || defButton();
        var body = el("div");
        node.components.forEach(function (td, ti) {
            var ta = el("textarea", { class: "form-control form-control-sm", placeholder: "Text" }); ta.rows = 2; ta.value = td.content || "";
            ta.addEventListener("input", function () { td.content = ta.value; touch(); });
            body.appendChild(el("div", { class: "mc-opt-row" }, [
                ta,
                node.components.length > 1 ? btn('<i class="fas fa-times"></i>', "btn-block btn-danger", function () { node.components.splice(ti, 1); rebuild(); }, { noconfirm: true }) : null
            ]));
        });
        if (node.components.length < 3) body.appendChild(btn("+ Text", "btn-block btn-outline-secondary", function () { node.components.push(defText()); rebuild(); }));
        body.appendChild(el("hr", { class: "my-2" }));
        body.appendChild(selectField("Accessory", [{ v: 2, t: "Button" }, { v: 11, t: "Thumbnail" }], node.accessory.type, function (v) {
            node.accessory = parseInt(v) === 2 ? defButton() : { type: 11, media: { url: "" } };
            rebuild();
        }));
        if (node.accessory.type === 2) body.appendChild(buttonEditor(node.accessory));
        else body.appendChild(field("Thumbnail URL", node.accessory.media && node.accessory.media.url, function (v) { node.accessory.media = { url: v }; touch(); }, { placeholder: "https://..." }));
        return body;
    }

    function containerBody(node) {
        node.components = node.components || [];
        var body = el("div");
        body.appendChild(colorField("Accent color", node.accent_color, function (v) {
            if (v == null) delete node.accent_color; else node.accent_color = v;
            touch();
        }));
        node.components.forEach(function (child, ci) { body.appendChild(renderNode(child, node.components, ci)); });
        body.appendChild(makeAddBar(node.components, "v2container"));
        return body;
    }

    function galleryBody(node) {
        node.items = node.items || [{ media: { url: "" } }];
        var body = el("div");
        node.items.forEach(function (it, ii) {
            it.media = it.media || { url: "" };
            body.appendChild(el("div", { class: "mc-opt-row" }, [
                inlineInput("Image / video URL", it.media.url, function (v) { it.media.url = v; touch(); }),
                inlineInput("Description (optional)", it.description, function (v) { if (v) it.description = v; else delete it.description; touch(); }),
                node.items.length > 1 ? btn('<i class="fas fa-times"></i>', "btn-block btn-danger", function () { node.items.splice(ii, 1); rebuild(); }, { noconfirm: true }) : null
            ]));
        });
        if (node.items.length < 10) body.appendChild(btn("+ Item", "btn-block btn-outline-secondary", function () { node.items.push({ media: { url: "" } }); rebuild(); }));
        return body;
    }


    // Finalize one interactive component's custom_id: link buttons carry none; locked components
    // (loaded without the templates- prefix) keep their id verbatim; everything else gets the
    // "templates-" prefix added to the user-entered suffix (blank stays blank for server auto-fill).
    function finalizeInteractive(c) {
        if (c.type === 2 && c.style === 5) { delete c.custom_id; delete c._locked; return; }
        if (c._locked) { delete c._locked; return; }
        delete c._locked;
        var s = (c.custom_id || "").trim();
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
        var id = c.custom_id || "";
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
        var out = [];
        comps.forEach(function (c) {
            if (c.type === 1) { if (c.components && c.components.length) out.push(c); }
            else if (c.type === 17) { c.components = pruneEmpty(c.components || []); if (c.components.length) out.push(c); }
            else out.push(c);
        });
        return out;
    }

    // Deep-clone the live model, finalize custom_ids, and prune empties for the outgoing payload.
    function buildComponents(comps) {
        var clone = JSON.parse(JSON.stringify(comps || []));
        finalizeWalk(clone);
        return pruneEmpty(clone);
    }

    function computeMessage() {
        var comps = buildComponents(state.components);
        if (currentMode === "componentsv2") return { message: { components: comps } };
        var msg = {};
        var content = val("embed-content"); if (content) msg.content = content;
        var embeds = buildEmbeds(); if (embeds.length) msg.embeds = embeds;
        if (comps.length) msg.components = comps;
        return { message: msg };
    }

    function renderPreview() {
        if (!els.preview) return;
        var msg = computeMessage().message;
        els.preview.innerHTML = "";
        if (msg.content) els.preview.appendChild(renderText(msg.content, "mc-content"));
        if (msg.embeds) msg.embeds.forEach(function (e) { els.preview.appendChild(renderEmbed(e)); });
        if (msg.components) renderComponents(msg.components, els.preview);
        if (!els.preview.childNodes.length) els.preview.appendChild(div("text-muted", "Nothing to preview yet."));
    }

    function renderEmbed(e) {
        var box = div("mc-embed");
        box.style.borderLeftColor = (typeof e.color === "number") ? intToHex(e.color) : "#4f545c";
        if (e.author && e.author.name) {
            var a = div("mc-embed-author");
            if (e.author.icon_url) a.appendChild(img(e.author.icon_url, "mc-embed-author-icon"));
            a.appendChild(span(e.author.name));
            box.appendChild(a);
        }
        if (e.title) box.appendChild(div("mc-embed-title", e.title)); // titles render no markdown on Discord
        if (e.description) { var d = div("mc-embed-desc"); d.innerHTML = formatMarkdown(e.description); box.appendChild(d); }
        if (e.fields && e.fields.length) {
            var grid = div("mc-embed-fields");
            e.fields.forEach(function (f) {
                var fd = div("mc-embed-field" + (f.inline ? " mc-inline" : ""));
                var fn = div("mc-embed-field-name"); fn.innerHTML = formatInline(f.name || "");
                var fv = div("mc-embed-field-value"); fv.innerHTML = formatMarkdown(f.value || "");
                fd.appendChild(fn); fd.appendChild(fv); grid.appendChild(fd);
            });
            box.appendChild(grid);
        }
        if (e.image && e.image.url) box.appendChild(img(e.image.url, "mc-embed-image"));
        if (e.thumbnail && e.thumbnail.url) box.appendChild(img(e.thumbnail.url, "mc-embed-thumb"));
        if (e.footer && (e.footer.text || e.timestamp)) {
            var f = div("mc-embed-footer");
            if (e.footer.icon_url) f.appendChild(img(e.footer.icon_url, "mc-embed-footer-icon"));
            var ftext = e.footer.text || "";
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
                var ct = div("mc-v2-container");
                ct.style.borderLeftColor = (typeof c.accent_color === "number") ? intToHex(c.accent_color) : "transparent";
                renderComponents(c.components || [], ct);
                return ct;
            case 9:
                var sec = div("mc-v2-section");
                var body = div("mc-v2-section-body");
                renderComponents(c.components || [], body);
                sec.appendChild(body);
                if (c.accessory) sec.appendChild(renderAccessory(c.accessory));
                return sec;
            case 1:
                var row = div("mc-v2-row");
                (c.components || []).forEach(function (ic) { row.appendChild(renderInteractive(ic)); });
                return row;
            case 14:
                var sep = div("mc-v2-separator");
                if (c.divider !== false) sep.classList.add("mc-divider");
                return sep;
            case 12:
                var g = div("mc-v2-gallery");
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
                var st = BUTTON_STYLES[c.style] || BUTTON_STYLES[2];
                var b = div("mc-v2-button", (c.emoji && c.emoji.name ? c.emoji.name + " " : "") + (c.label || (c.style === 5 ? "Link" : "Button")));
                b.style.backgroundColor = st.bg; b.style.color = st.fg;
                if (c.style === 5) { // link buttons show an external-link indicator, like Discord
                    var icon = document.createElement("i");
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

    function renderText(text, cls) { var d = div(cls || ""); d.innerHTML = formatMarkdown(text); return d; }


    function loadMessage() {
        var link = document.getElementById("mc-edit-link").value.trim();
        var errEl = document.getElementById("mc-load-error");
        errEl.className = "small mt-1 text-danger";
        errEl.textContent = "";
        if (!link) { errEl.textContent = "Paste a message link first."; return; }

        fetch(base() + "/load?link=" + encodeURIComponent(link), { credentials: "same-origin" })
            .then(function (r) { return r.json().then(function (j) { return { ok: r.ok, body: j }; }); })
            .then(function (res) {
                if (!res.ok || res.body.error) { errEl.textContent = res.body.error || "Failed to load message."; return; }
                populateFromMessage(res.body);
                if (!res.body.author_is_bot) {
                    errEl.className = "small mt-1 text-warning";
                    errEl.textContent = "This message was not sent by YAGPDB, so it can't be edited (switch to Create to send it as a new message).";
                    loadedForEdit = false; els.submit.disabled = true; show(els.editHint, true);
                } else {
                    loadedForEdit = true; els.submit.disabled = false; show(els.editHint, false);
                }
            })
            .catch(function (e) { errEl.textContent = "Failed to load message: " + e.message; });
    }

    function populateFromMessage(resp) {
        var payload = resp.payload || {};
        var mode = resp.mode === "componentsv2" ? "componentsv2" : "normal";
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
        rebuild();
    }


    function el(tag, attrs, children) {
        var e = document.createElement(tag);
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
        var b = document.createElement("button");
        b.type = "button"; b.className = "btn " + cls; b.innerHTML = html;
        if (opts.noconfirm) b.setAttribute("noconfirm", "true");
        if (opts.title) b.title = opts.title;
        b.addEventListener("click", onclick);
        return b;
    }

    function field(labelText, value, oninput, opts) {
        opts = opts || {};
        var input = document.createElement(opts.textarea ? "textarea" : "input");
        input.className = "form-control form-control-sm";
        if (opts.textarea) input.rows = opts.rows || 2;
        if (opts.placeholder) input.placeholder = opts.placeholder;
        if (opts.maxlength) input.maxLength = opts.maxlength;
        input.value = value == null ? "" : value;
        input.addEventListener("input", function () { oninput(input.value); });
        var children = [el("label", { class: "small mb-1" }, [labelText]), input];
        if (opts.help) children.push(el("small", { class: "form-text text-muted" }, [opts.help]));
        return el("div", { class: "form-group mb-2" }, children);
    }

    function readonlyField(labelText, value, help) {
        var input = el("input", { class: "form-control form-control-sm" }); input.value = value || ""; input.disabled = true;
        var children = [el("label", { class: "small mb-1" }, [labelText]), input];
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
        return field("Custom ID (optional)", node.custom_id, function (v) { node.custom_id = v; touch(); },
            { placeholder: "auto-generated", maxlength: MAX_SUFFIX, help: '"templates-" is added automatically so it can trigger a custom command.' });
    }

    function inlineInput(placeholder, value, oninput) {
        var i = document.createElement("input");
        i.className = "form-control form-control-sm"; i.placeholder = placeholder;
        i.value = value == null ? "" : value;
        i.addEventListener("input", function () { oninput(i.value); });
        return i;
    }

    function numField(labelText, value, oninput) {
        var i = document.createElement("input");
        i.type = "number"; i.className = "form-control form-control-sm"; i.min = 0;
        i.value = value == null ? "" : value;
        i.addEventListener("input", function () { oninput(i.value === "" ? null : parseInt(i.value)); });
        return el("div", { class: "form-group mb-2" }, [el("label", { class: "small mb-1" }, [labelText]), i]);
    }

    function selectField(labelText, options, value, onchange) {
        var sel = document.createElement("select");
        sel.className = "form-control form-control-sm";
        options.forEach(function (o) {
            var op = document.createElement("option");
            op.value = String(o.v); op.textContent = o.t;
            if (String(o.v) === String(value)) op.selected = true;
            sel.appendChild(op);
        });
        sel.addEventListener("change", function () { onchange(sel.value); });
        return el("div", { class: "form-group mb-2" }, [el("label", { class: "small mb-1" }, [labelText]), sel]);
    }

    function show(elm, on) { if (elm) elm.style.display = on ? "" : "none"; }
    function setVal(id, v) { var e = document.getElementById(id); if (e) e.value = v; }
    function setChecked(id, v) { var e = document.getElementById(id); if (e) e.checked = !!v; }
    function hexToInt(hex) { return hex ? (parseInt(hex.replace("#", ""), 16) || 0) : 0; }
    function intToHex(i) { var s = (i & 0xffffff).toString(16); while (s.length < 6) s = "0" + s; return "#" + s; }

    function div(cls, text) { var d = document.createElement("div"); if (cls) d.className = cls; if (text != null) d.textContent = text; return d; }
    function span(text) { var s = document.createElement("span"); s.textContent = text; return s; }
    function img(url, cls) {
        var i = document.createElement("img");
        i.src = url; if (cls) i.className = cls;
        i.onerror = function () { i.style.display = "none"; };
        return i;
    }

    function esc(s) { return String(s).replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;"); }

    // Inline-only Discord markdown: spoilers, bold/italic/underline/strikethrough, masked links.
    // Operates on already-HTML-escaped text. Used where Discord only formats inline (e.g. field names).
    function mdInline(s) {
        s = s.replace(/\|\|([\s\S]+?)\|\|/g, '<span class="mc-md-spoiler">$1</span>');
        s = s.replace(/\*\*\*([\s\S]+?)\*\*\*/g, "<strong><em>$1</em></strong>");
        s = s.replace(/\*\*([\s\S]+?)\*\*/g, "<strong>$1</strong>");
        s = s.replace(/\*([^\s*][\s\S]*?)\*/g, "<em>$1</em>");
        s = s.replace(/__([\s\S]+?)__/g, "<u>$1</u>");
        s = s.replace(/_([^_]+?)_/g, "<em>$1</em>");
        s = s.replace(/~~([\s\S]+?)~~/g, "<s>$1</s>");
        s = s.replace(/\[([^\]]+)\]\((https?:\/\/[^\s)]+)\)/g, '<a href="$2" target="_blank" rel="noopener">$1</a>');
        return s;
    }

    // Inline-only renderer for fields that don't support block markdown.
    function formatInline(text) { return mdInline(esc(String(text))); }

    // Full Discord markdown: code blocks/inline code, headers (#, ##, ###), subtext (-#),
    // blockquotes (>), bullet/numbered lists, plus all inline marks. Used for message content,
    // Text Display components, embed descriptions and field values.
    function formatMarkdown(text) {
        var src = String(text);
        var codes = [];
        // Pull out code blocks and inline code first so their contents aren't formatted.
        src = src.replace(/```(?:[a-zA-Z0-9_+\-]*\n)?([\s\S]*?)```/g, function (_, code) {
            codes.push('<pre class="mc-md-codeblock"><code>' + esc(code.replace(/\n$/, "")) + "</code></pre>");
            return " " + (codes.length - 1) + " ";
        });
        src = src.replace(/`([^`\n]+)`/g, function (_, code) {
            codes.push('<code class="mc-md-code">' + esc(code) + "</code>");
            return " " + (codes.length - 1) + " ";
        });
        src = esc(src);

        var parts = [], prevInline = false, list = null;
        function pushBlock(html) { flushList(); parts.push(html); prevInline = false; }
        function pushInline(html) { flushList(); if (prevInline) parts.push("<br>"); parts.push(html); prevInline = true; }
        function flushList() {
            if (!list) return;
            parts.push("<" + list.type + ' class="mc-md-list">' + list.items.map(function (it) { return "<li>" + mdInline(it) + "</li>"; }).join("") + "</" + list.type + ">");
            list = null; prevInline = false;
        }

        src.split("\n").forEach(function (line) {
            var ul = /^\s*[-*]\s+(.*)$/.exec(line);
            var ol = /^\s*\d+\.\s+(.*)$/.exec(line);
            if (ul) { if (!list || list.type !== "ul") { flushList(); list = { type: "ul", items: [] }; } list.items.push(ul[1]); return; }
            if (ol) { if (!list || list.type !== "ol") { flushList(); list = { type: "ol", items: [] }; } list.items.push(ol[1]); return; }
            var h = /^(#{1,3})\s+(.*)$/.exec(line);
            var sub = /^-#\s+(.*)$/.exec(line);
            var bq = /^&gt;\s?(.*)$/.exec(line); // ">" was HTML-escaped above
            if (h) { pushBlock('<div class="mc-md-h mc-md-h' + h[1].length + '">' + mdInline(h[2]) + "</div>"); return; }
            if (sub) { pushBlock('<div class="mc-md-subtext">' + mdInline(sub[1]) + "</div>"); return; }
            if (bq) { pushBlock('<blockquote class="mc-md-quote">' + mdInline(bq[1]) + "</blockquote>"); return; }
            pushInline(mdInline(line));
        });
        flushList();

        var html = parts.join("");
        return html.replace(/ (\d+) /g, function (_, n) { return codes[+n]; });
    }

    init();
})();
