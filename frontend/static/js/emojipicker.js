// Emoji picker shared by dashboard pages. Server emojis are read from the closest
// [data-guild-emojis] element, a JSON list of {id, name, animated}.
(function () {
    "use strict";

    if (window.YAGEmojiPicker) return;

    let CUSTOM_EMOJI = /^<(a?):(\w{2,32}):(\d{15,22})>$/;
    let dataPromise = null;
    let openPicker = null;

    function loadUnicodeData() {
        if (window.YAGEmojiData) return Promise.resolve(window.YAGEmojiData);
        if (dataPromise) return dataPromise;
        dataPromise = new Promise(function (resolve, reject) {
            let script = document.createElement("script");
            script.src = "/static/js/emojidata.js";
            script.onload = function () { resolve(window.YAGEmojiData || []); };
            script.onerror = function () { dataPromise = null; reject(new Error("failed loading emojis")); };
            document.head.appendChild(script);
        });
        return dataPromise;
    }

    function guildEmojis(anchor) {
        let holder = (anchor && anchor.closest("[data-guild-emojis]")) || document.querySelector("[data-guild-emojis]");
        if (!holder) return [];
        try {
            return (JSON.parse(holder.getAttribute("data-guild-emojis")) || []).map(function (e) {
                return { id: String(e.id), name: e.name, animated: !!e.animated };
            });
        } catch (e) {
            return [];
        }
    }

    function emojiURL(emoji) {
        return "https://cdn.discordapp.com/emojis/" + emoji.id + "." + (emoji.animated ? "gif" : "webp") + "?size=48";
    }

    function format(emoji) {
        if (!emoji) return "";
        if (emoji.id) return "<" + (emoji.animated ? "a" : "") + ":" + emoji.name + ":" + emoji.id + ">";
        return emoji.name || "";
    }

    function parse(text) {
        text = (text || "").trim();
        if (!text) return null;
        let match = CUSTOM_EMOJI.exec(text);
        if (match) return { id: match[3], name: match[2], animated: match[1] === "a" };
        return { name: text };
    }

    // render returns a node showing the emoji, an image for custom ones.
    function render(emoji, cls) {
        if (emoji && emoji.id) {
            let img = document.createElement("img");
            img.src = emojiURL(emoji);
            img.alt = ":" + emoji.name + ":";
            img.title = ":" + emoji.name + ":";
            img.className = cls || "";
            img.style.width = "1.375em";
            img.style.height = "1.375em";
            img.style.objectFit = "contain";
            img.style.verticalAlign = "-0.3em";
            return img;
        }
        let span = document.createElement("span");
        span.className = cls || "";
        span.textContent = emoji ? emoji.name : "";
        return span;
    }

    function close() {
        if (!openPicker) return;
        let picker = openPicker;
        openPicker = null;
        document.removeEventListener("mousedown", picker.onOutside, true);
        document.removeEventListener("keydown", picker.onKey, true);
        window.removeEventListener("resize", picker.onResize);
        picker.el.remove();
        if (picker.anchor && document.body.contains(picker.anchor)) picker.anchor.focus();
    }

    function position(picker) {
        let rect = picker.anchor.getBoundingClientRect();
        let width = Math.min(340, document.documentElement.clientWidth - 32);
        let left = Math.min(rect.left, document.documentElement.clientWidth - width - 16);
        picker.el.style.width = width + "px";
        picker.el.style.left = Math.max(16, left) + window.scrollX + "px";
        picker.el.style.top = rect.bottom + window.scrollY + 4 + "px";
    }

    function emojiButton(emoji, label, onPick) {
        let b = document.createElement("button");
        b.type = "button";
        b.className = "btn btn-link p-1";
        b.style.width = "2.5rem";
        b.style.height = "2.5rem";
        b.style.fontSize = "1.35rem";
        b.style.lineHeight = "1";
        b.setAttribute("noconfirm", "true");
        b.title = label;
        b.setAttribute("aria-label", label);
        b.appendChild(render(emoji));
        b.addEventListener("click", function () { onPick(emoji); close(); });
        return b;
    }

    function section(title) {
        let wrap = document.createElement("div");
        let heading = document.createElement("div");
        heading.className = "small font-weight-bold text-uppercase mt-1 mb-1";
        heading.textContent = title;
        let grid = document.createElement("div");
        grid.className = "d-flex flex-wrap";
        wrap.appendChild(heading);
        wrap.appendChild(grid);
        return { wrap: wrap, grid: grid };
    }

    // open shows the picker under anchor and calls onPick({id, name, animated}) with the choice, or onClear for "No emoji".
    function open(anchor, onPick, onClear) {
        if (openPicker && openPicker.anchor === anchor) { close(); return; }
        close();

        let el = document.createElement("div");
        el.className = "dropdown-menu show p-2";
        el.setAttribute("role", "dialog");
        el.setAttribute("aria-label", "Emoji picker");
        el.style.position = "absolute";
        el.style.zIndex = "1060";

        let search = document.createElement("input");
        search.type = "search";
        search.className = "form-control form-control-sm mb-2";
        search.placeholder = "Search emojis";
        search.setAttribute("aria-label", "Search emojis");
        el.appendChild(search);

        if (onClear) {
            let clear = document.createElement("button");
            clear.type = "button";
            clear.className = "btn btn-link btn-sm p-1 mb-1";
            clear.setAttribute("noconfirm", "true");
            clear.textContent = "No emoji";
            clear.addEventListener("click", function () { onClear(); close(); });
            el.appendChild(clear);
        }

        let list = document.createElement("div");
        list.style.maxHeight = "260px";
        list.style.overflowY = "auto";
        el.appendChild(list);

        let items = [];
        let custom = guildEmojis(anchor);
        if (custom.length) {
            let s = section("This server");
            custom.forEach(function (emoji) {
                let b = emojiButton(emoji, ":" + emoji.name + ":", onPick);
                s.grid.appendChild(b);
                items.push({ el: b, terms: emoji.name.toLowerCase(), section: s.wrap });
            });
            list.appendChild(s.wrap);
        }

        let standard = section("Emojis");
        let status = document.createElement("div");
        status.className = "small p-1";
        status.textContent = "Loading...";
        standard.grid.appendChild(status);
        list.appendChild(standard.wrap);

        let empty = document.createElement("div");
        empty.className = "small p-1";
        empty.textContent = "No emojis found.";
        empty.style.display = "none";
        list.appendChild(empty);

        function filter() {
            let query = search.value.trim().toLowerCase().replace(/^:|:$/g, "");
            let shown = new Set();
            items.forEach(function (item) {
                let match = !query || item.terms.indexOf(query) !== -1;
                item.el.style.display = match ? "" : "none";
                if (match) shown.add(item.section);
            });
            list.querySelectorAll(":scope > div").forEach(function (wrap) {
                if (wrap === empty || !wrap.querySelector("button")) return;
                wrap.style.display = shown.has(wrap) ? "" : "none";
            });
            empty.style.display = items.length && !shown.size ? "" : "none";
        }

        loadUnicodeData().then(function (data) {
            if (!el.isConnected) return;
            status.remove();
            let fragment = document.createDocumentFragment();
            data.forEach(function (entry) {
                let names = entry[1].split(" ");
                let b = emojiButton({ name: entry[0] }, ":" + names[0] + ":", onPick);
                fragment.appendChild(b);
                items.push({ el: b, terms: entry[1].toLowerCase(), section: standard.wrap });
            });
            standard.grid.appendChild(fragment);
            filter();
        }).catch(function () {
            status.textContent = "Couldn't load emojis, you can still type or paste one.";
        });

        search.addEventListener("input", filter);

        let picker = {
            el: el,
            anchor: anchor,
            onOutside: function (e) { if (!el.contains(e.target) && !anchor.contains(e.target)) close(); },
            onKey: function (e) { if (e.key === "Escape") { e.preventDefault(); close(); } },
            onResize: function () { position(picker); }
        };
        openPicker = picker;

        document.body.appendChild(el);
        el.style.color = getComputedStyle(search).color;
        position(picker);
        document.addEventListener("mousedown", picker.onOutside, true);
        document.addEventListener("keydown", picker.onKey, true);
        window.addEventListener("resize", picker.onResize);
        search.focus();
    }

    // trigger returns a button that opens the picker.
    function trigger(onPick, cls) {
        let b = document.createElement("button");
        b.type = "button";
        b.className = "btn " + (cls || "btn-outline-primary");
        b.setAttribute("noconfirm", "true");
        b.title = "Pick an emoji";
        b.setAttribute("aria-label", "Pick an emoji");
        b.setAttribute("aria-haspopup", "dialog");
        b.innerHTML = '<i class="far fa-smile" aria-hidden="true"></i>';
        b.addEventListener("click", function () { open(b, onPick); });
        return b;
    }

    // insertAtCursor puts the emoji's text where the caret is in a textarea or input.
    function insertAtCursor(input, emoji) {
        let text = format(emoji);
        let start = input.selectionStart == null ? input.value.length : input.selectionStart;
        let end = input.selectionEnd == null ? start : input.selectionEnd;
        input.value = input.value.slice(0, start) + text + input.value.slice(end);
        input.selectionStart = input.selectionEnd = start + text.length;
        input.dispatchEvent(new Event("input", { bubbles: true }));
    }

    window.YAGEmojiPicker = {
        open: open,
        close: close,
        trigger: trigger,
        parse: parse,
        format: format,
        render: render,
        insertAtCursor: insertAtCursor
    };

    // Inputs marked data-emoji-picker get a picker button: "replace" sets the value, anything else inserts at the caret.
    function enhance(root) {
        (root || document).querySelectorAll("[data-emoji-picker]:not([data-emoji-picker-ready])").forEach(function (input) {
            input.setAttribute("data-emoji-picker-ready", "true");
            let replace = input.getAttribute("data-emoji-picker") === "replace";
            let button = trigger(function (emoji) {
                if (replace) {
                    input.value = format(emoji);
                    input.dispatchEvent(new Event("input", { bubbles: true }));
                    input.dispatchEvent(new Event("change", { bubbles: true }));
                } else {
                    insertAtCursor(input, emoji);
                }
            }, "btn-outline-primary");

            let group = input.closest(".input-group");
            if (!group) {
                group = document.createElement("div");
                group.className = "input-group";
                input.parentNode.insertBefore(group, input);
                group.appendChild(input);
            }
            let append = document.createElement("div");
            append.className = "input-group-append";
            append.appendChild(button);
            group.appendChild(append);
        });
    }

    window.YAGEmojiPicker.enhance = enhance;
})();
