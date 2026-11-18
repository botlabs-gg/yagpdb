// Renders Discord components and markdown for dashboard previews, shared by the message creator and server previews.
(function () {
    "use strict";

    let BUTTON_STYLES = {
        1: { bg: "#5865f2", fg: "#fff" },
        2: { bg: "#4f545c", fg: "#fff" },
        3: { bg: "#248046", fg: "#fff" },
        4: { bg: "#da373c", fg: "#fff" },
        5: { bg: "#4f545c", fg: "#fff" }
    };

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
                let b = div("mc-v2-button");
                if (c.emoji && (c.emoji.id || c.emoji.name)) b.appendChild(emojiNode(c.emoji));
                b.appendChild(span(c.label || (c.style === 5 ? "Link" : "Button")));
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

    function intToHex(i) { let s = (i & 0xffffff).toString(16); while (s.length < 6) s = "0" + s; return "#" + s; }

    function div(cls, text) { let d = document.createElement("div"); if (cls) d.className = cls; if (text != null) d.textContent = text; return d; }
    function span(text) { let s = document.createElement("span"); s.textContent = text; return s; }
    function img(url, cls) {
        let i = document.createElement("img");
        i.src = url; if (cls) i.className = cls;
        i.onerror = function () { i.style.display = "none"; };
        return i;
    }

    function emojiNode(emoji) {
        if (window.YAGEmojiPicker) return YAGEmojiPicker.render(emoji, "mc-emoji");
        return span(emoji.name || "");
    }

    // ---- Markdown renderer ----
    let INLINE_RULES = [
        { regex: /\\([\\*_~`|>#\[\]()\-])/, build: function (match) { return document.createTextNode(match[1]); } },
        { regex: /\|\|([\s\S]+?)\|\|/, tag: "span", className: "mc-md-spoiler", recurse: true },
        { regex: /\*\*\*([\s\S]+?)\*\*\*/, build: buildBoldItalicNode },
        { regex: /\*\*([\s\S]+?)\*\*/, tag: "strong", recurse: true },
        { regex: /__([\s\S]+?)__/, tag: "u", recurse: true },
        { regex: /\*([^\s*][\s\S]*?)\*/, tag: "em", recurse: true },
        { regex: /_([^_]+?)_/, tag: "em", recurse: true },
        { regex: /~~([\s\S]+?)~~/, tag: "s", recurse: true },
        { regex: /`([^`\n]+)`/, tag: "code", className: "mc-md-code", recurse: false },
        { regex: /\[([^\]]+)\]\((https?:\/\/[^\s)]+)\)/, build: buildLinkNode },
        { regex: /<(a?):(\w{2,32}):(\d{15,22})>/, build: buildCustomEmojiNode }
    ];

    function buildCustomEmojiNode(match) {
        return emojiNode({ id: match[3], name: match[2], animated: match[1] === "a" });
    }

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

    window.YAGDiscordPreview = {
        renderComponents: renderComponents,
        renderComponent: renderComponent,
        renderText: renderText,
        renderMarkdown: renderMarkdown,
        renderInlineMarkdown: renderInlineMarkdown,
        emojiNode: emojiNode
    };
})();
