/*! highlight.js v9.15.10 | BSD3 License | git.io/hljslicense */
!(function (e) {
  var n =
    ("object" == typeof window && window) || ("object" == typeof self && self);
  "undefined" == typeof exports || exports.nodeType
    ? n &&
      ((n.hljs = e({})),
      "function" == typeof define &&
        define.amd &&
        define([], function () {
          return n.hljs;
        }))
    : e(exports);
})(function (a) {
  var f = [],
    u = Object.keys,
    N = {},
    c = {},
    n = /^(no-?highlight|plain|text)$/i,
    s = /\blang(?:uage)?-([\w-]+)\b/i,
    t = /((^(<[^>]+>|\t|)+|(?:\n)))/gm,
    r = {
      case_insensitive: "cI",
      lexemes: "l",
      contains: "c",
      keywords: "k",
      subLanguage: "sL",
      className: "cN",
      begin: "b",
      beginKeywords: "bK",
      end: "e",
      endsWithParent: "eW",
      illegal: "i",
      excludeBegin: "eB",
      excludeEnd: "eE",
      returnBegin: "rB",
      returnEnd: "rE",
      relevance: "r",
      variants: "v",
      IDENT_RE: "IR",
      UNDERSCORE_IDENT_RE: "UIR",
      NUMBER_RE: "NR",
      C_NUMBER_RE: "CNR",
      BINARY_NUMBER_RE: "BNR",
      RE_STARTERS_RE: "RSR",
      BACKSLASH_ESCAPE: "BE",
      APOS_STRING_MODE: "ASM",
      QUOTE_STRING_MODE: "QSM",
      PHRASAL_WORDS_MODE: "PWM",
      C_LINE_COMMENT_MODE: "CLCM",
      C_BLOCK_COMMENT_MODE: "CBCM",
      HASH_COMMENT_MODE: "HCM",
      NUMBER_MODE: "NM",
      C_NUMBER_MODE: "CNM",
      BINARY_NUMBER_MODE: "BNM",
      CSS_NUMBER_MODE: "CSSNM",
      REGEXP_MODE: "RM",
      TITLE_MODE: "TM",
      UNDERSCORE_TITLE_MODE: "UTM",
      COMMENT: "C",
      beginRe: "bR",
      endRe: "eR",
      illegalRe: "iR",
      lexemesRe: "lR",
      terminators: "t",
      terminator_end: "tE",
    },
    b = "</span>",
    h = {
      classPrefix: "hljs-",
      tabReplace: null,
      useBR: !1,
      languages: void 0,
    };
  function _(e) {
    return e.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;");
  }
  function E(e) {
    return e.nodeName.toLowerCase();
  }
  function v(e, n) {
    var t = e && e.exec(n);
    return t && 0 === t.index;
  }
  function l(e) {
    return n.test(e);
  }
  function g(e) {
    var n,
      t = {},
      r = Array.prototype.slice.call(arguments, 1);
    for (n in e) t[n] = e[n];
    return (
      r.forEach(function (e) {
        for (n in e) t[n] = e[n];
      }),
      t
    );
  }
  function R(e) {
    var a = [];
    return (
      (function e(n, t) {
        for (var r = n.firstChild; r; r = r.nextSibling)
          3 === r.nodeType
            ? (t += r.nodeValue.length)
            : 1 === r.nodeType &&
              (a.push({ event: "start", offset: t, node: r }),
              (t = e(r, t)),
              E(r).match(/br|hr|img|input/) ||
                a.push({ event: "stop", offset: t, node: r }));
        return t;
      })(e, 0),
      a
    );
  }
  function i(e) {
    if (r && !e.langApiRestored) {
      for (var n in ((e.langApiRestored = !0), r)) e[n] && (e[r[n]] = e[n]);
      (e.c || []).concat(e.v || []).forEach(i);
    }
  }
  function m(o) {
    function s(e) {
      return (e && e.source) || e;
    }
    function c(e, n) {
      return new RegExp(s(e), "m" + (o.cI ? "i" : "") + (n ? "g" : ""));
    }
    !(function n(t, e) {
      if (!t.compiled) {
        if (((t.compiled = !0), (t.k = t.k || t.bK), t.k)) {
          function r(t, e) {
            o.cI && (e = e.toLowerCase()),
              e.split(" ").forEach(function (e) {
                var n = e.split("|");
                a[n[0]] = [t, n[1] ? Number(n[1]) : 1];
              });
          }
          var a = {};
          "string" == typeof t.k
            ? r("keyword", t.k)
            : u(t.k).forEach(function (e) {
                r(e, t.k[e]);
              }),
            (t.k = a);
        }
        (t.lR = c(t.l || /\w+/, !0)),
          e &&
            (t.bK && (t.b = "\\b(" + t.bK.split(" ").join("|") + ")\\b"),
            t.b || (t.b = /\B|\b/),
            (t.bR = c(t.b)),
            t.endSameAsBegin && (t.e = t.b),
            t.e || t.eW || (t.e = /\B|\b/),
            t.e && (t.eR = c(t.e)),
            (t.tE = s(t.e) || ""),
            t.eW && e.tE && (t.tE += (t.e ? "|" : "") + e.tE)),
          t.i && (t.iR = c(t.i)),
          null == t.r && (t.r = 1),
          t.c || (t.c = []),
          (t.c = Array.prototype.concat.apply(
            [],
            t.c.map(function (e) {
              return (function (n) {
                return (
                  n.v &&
                    !n.cached_variants &&
                    (n.cached_variants = n.v.map(function (e) {
                      return g(n, { v: null }, e);
                    })),
                  n.cached_variants || (n.eW && [g(n)]) || [n]
                );
              })("self" === e ? t : e);
            })
          )),
          t.c.forEach(function (e) {
            n(e, t);
          }),
          t.starts && n(t.starts, e);
        var i = t.c
          .map(function (e) {
            return e.bK ? "\\.?(?:" + e.b + ")\\.?" : e.b;
          })
          .concat([t.tE, t.i])
          .map(s)
          .filter(Boolean);
        t.t = i.length
          ? c(
              (function (e, n) {
                for (
                  var t = /\[(?:[^\\\]]|\\.)*\]|\(\??|\\([1-9][0-9]*)|\\./,
                    r = 0,
                    a = "",
                    i = 0;
                  i < e.length;
                  i++
                ) {
                  var o = r,
                    c = s(e[i]);
                  for (0 < i && (a += n); 0 < c.length; ) {
                    var u = t.exec(c);
                    if (null == u) {
                      a += c;
                      break;
                    }
                    (a += c.substring(0, u.index)),
                      (c = c.substring(u.index + u[0].length)),
                      "\\" == u[0][0] && u[1]
                        ? (a += "\\" + String(Number(u[1]) + o))
                        : ((a += u[0]), "(" == u[0] && r++);
                  }
                }
                return a;
              })(i, "|"),
              !0
            )
          : {
              exec: function () {
                return null;
              },
            };
      }
    })(o);
  }
  function C(e, n, i, t) {
    function c(e, n, t, r) {
      var a = '<span class="' + (r ? "" : h.classPrefix);
      return e ? (a += e + '">') + n + (t ? "" : b) : n;
    }
    function o() {
      (E +=
        null != l.sL
          ? (function () {
              var e = "string" == typeof l.sL;
              if (e && !N[l.sL]) return _(g);
              var n = e
                ? C(l.sL, g, !0, f[l.sL])
                : O(g, l.sL.length ? l.sL : void 0);
              return (
                0 < l.r && (R += n.r),
                e && (f[l.sL] = n.top),
                c(n.language, n.value, !1, !0)
              );
            })()
          : (function () {
              var e, n, t, r, a, i, o;
              if (!l.k) return _(g);
              for (r = "", n = 0, l.lR.lastIndex = 0, t = l.lR.exec(g); t; )
                (r += _(g.substring(n, t.index))),
                  (a = l),
                  (i = t),
                  void 0,
                  (o = s.cI ? i[0].toLowerCase() : i[0]),
                  (e = a.k.hasOwnProperty(o) && a.k[o])
                    ? ((R += e[1]), (r += c(e[0], _(t[0]))))
                    : (r += _(t[0])),
                  (n = l.lR.lastIndex),
                  (t = l.lR.exec(g));
              return r + _(g.substr(n));
            })()),
        (g = "");
    }
    function u(e) {
      (E += e.cN ? c(e.cN, "", !0) : ""),
        (l = Object.create(e, { parent: { value: l } }));
    }
    function r(e, n) {
      if (((g += e), null == n)) return o(), 0;
      var t = (function (e, n) {
        var t, r, a;
        for (t = 0, r = n.c.length; t < r; t++)
          if (v(n.c[t].bR, e))
            return (
              n.c[t].endSameAsBegin &&
                (n.c[t].eR =
                  ((a = n.c[t].bR.exec(e)[0]),
                  new RegExp(
                    a.replace(/[-\/\\^$*+?.()|[\]{}]/g, "\\$&"),
                    "m"
                  ))),
              n.c[t]
            );
      })(n, l);
      if (t)
        return (
          t.skip ? (g += n) : (t.eB && (g += n), o(), t.rB || t.eB || (g = n)),
          u(t),
          t.rB ? 0 : n.length
        );
      var r = (function e(n, t) {
        if (v(n.eR, t)) {
          for (; n.endsParent && n.parent; ) n = n.parent;
          return n;
        }
        if (n.eW) return e(n.parent, t);
      })(l, n);
      if (r) {
        var a = l;
        for (
          a.skip ? (g += n) : (a.rE || a.eE || (g += n), o(), a.eE && (g = n));
          l.cN && (E += b),
            l.skip || l.sL || (R += l.r),
            (l = l.parent) !== r.parent;

        );
        return (
          r.starts && (r.endSameAsBegin && (r.starts.eR = r.eR), u(r.starts)),
          a.rE ? 0 : n.length
        );
      }
      if (
        (function (e, n) {
          return !i && v(n.iR, e);
        })(n, l)
      )
        throw new Error(
          'Illegal lexeme "' + n + '" for mode "' + (l.cN || "<unnamed>") + '"'
        );
      return (g += n), n.length || 1;
    }
    var s = B(e);
    if (!s) throw new Error('Unknown language: "' + e + '"');
    m(s);
    var a,
      l = t || s,
      f = {},
      E = "";
    for (a = l; a !== s; a = a.parent) a.cN && (E = c(a.cN, "", !0) + E);
    var g = "",
      R = 0;
    try {
      for (var d, p, M = 0; (l.t.lastIndex = M), (d = l.t.exec(n)); )
        (p = r(n.substring(M, d.index), d[0])), (M = d.index + p);
      for (r(n.substr(M)), a = l; a.parent; a = a.parent) a.cN && (E += b);
      return { r: R, value: E, language: e, top: l };
    } catch (e) {
      if (e.message && -1 !== e.message.indexOf("Illegal"))
        return { r: 0, value: _(n) };
      throw e;
    }
  }
  function O(t, e) {
    e = e || h.languages || u(N);
    var r = { r: 0, value: _(t) },
      a = r;
    return (
      e
        .filter(B)
        .filter(M)
        .forEach(function (e) {
          var n = C(e, t, !1);
          (n.language = e),
            n.r > a.r && (a = n),
            n.r > r.r && ((a = r), (r = n));
        }),
      a.language && (r.second_best = a),
      r
    );
  }
  function d(e) {
    return h.tabReplace || h.useBR
      ? e.replace(t, function (e, n) {
          return h.useBR && "\n" === e
            ? "<br>"
            : h.tabReplace
            ? n.replace(/\t/g, h.tabReplace)
            : "";
        })
      : e;
  }
  function o(e) {
    var n,
      t,
      r,
      a,
      i,
      o = (function (e) {
        var n,
          t,
          r,
          a,
          i = e.className + " ";
        if (
          ((i += e.parentNode ? e.parentNode.className : ""), (t = s.exec(i)))
        )
          return B(t[1]) ? t[1] : "no-highlight";
        for (n = 0, r = (i = i.split(/\s+/)).length; n < r; n++)
          if (l((a = i[n])) || B(a)) return a;
      })(e);
    l(o) ||
      (h.useBR
        ? ((n = document.createElementNS(
            "http://www.w3.org/1999/xhtml",
            "div"
          )).innerHTML = e.innerHTML
            .replace(/\n/g, "")
            .replace(/<br[ \/]*>/g, "\n"))
        : (n = e),
      (i = n.textContent),
      (r = o ? C(o, i, !0) : O(i)),
      (t = R(n)).length &&
        (((a = document.createElementNS(
          "http://www.w3.org/1999/xhtml",
          "div"
        )).innerHTML = r.value),
        (r.value = (function (e, n, t) {
          var r = 0,
            a = "",
            i = [];
          function o() {
            return e.length && n.length
              ? e[0].offset !== n[0].offset
                ? e[0].offset < n[0].offset
                  ? e
                  : n
                : "start" === n[0].event
                ? e
                : n
              : e.length
              ? e
              : n;
          }
          function c(e) {
            a +=
              "<" +
              E(e) +
              f.map
                .call(e.attributes, function (e) {
                  return (
                    " " +
                    e.nodeName +
                    '="' +
                    _(e.value).replace('"', "&quot;") +
                    '"'
                  );
                })
                .join("") +
              ">";
          }
          function u(e) {
            a += "</" + E(e) + ">";
          }
          function s(e) {
            ("start" === e.event ? c : u)(e.node);
          }
          for (; e.length || n.length; ) {
            var l = o();
            if (
              ((a += _(t.substring(r, l[0].offset))),
              (r = l[0].offset),
              l === e)
            ) {
              for (
                i.reverse().forEach(u);
                s(l.splice(0, 1)[0]),
                  (l = o()) === e && l.length && l[0].offset === r;

              );
              i.reverse().forEach(c);
            } else
              "start" === l[0].event ? i.push(l[0].node) : i.pop(),
                s(l.splice(0, 1)[0]);
          }
          return a + _(t.substr(r));
        })(t, R(a), i))),
      (r.value = d(r.value)),
      (e.innerHTML = r.value),
      (e.className = (function (e, n, t) {
        var r = n ? c[n] : t,
          a = [e.trim()];
        return (
          e.match(/\bhljs\b/) || a.push("hljs"),
          -1 === e.indexOf(r) && a.push(r),
          a.join(" ").trim()
        );
      })(e.className, o, r.language)),
      (e.result = { language: r.language, re: r.r }),
      r.second_best &&
        (e.second_best = {
          language: r.second_best.language,
          re: r.second_best.r,
        }));
  }
  function p() {
    if (!p.called) {
      p.called = !0;
      var e = document.querySelectorAll("pre code");
      f.forEach.call(e, o);
    }
  }
  function B(e) {
    return (e = (e || "").toLowerCase()), N[e] || N[c[e]];
  }
  function M(e) {
    var n = B(e);
    return n && !n.disableAutodetect;
  }
  return (
    (a.highlight = C),
    (a.highlightAuto = O),
    (a.fixMarkup = d),
    (a.highlightBlock = o),
    (a.configure = function (e) {
      h = g(h, e);
    }),
    (a.initHighlighting = p),
    (a.initHighlightingOnLoad = function () {
      addEventListener("DOMContentLoaded", p, !1),
        addEventListener("load", p, !1);
    }),
    (a.registerLanguage = function (n, e) {
      var t = (N[n] = e(a));
      i(t),
        t.aliases &&
          t.aliases.forEach(function (e) {
            c[e] = n;
          });
    }),
    (a.listLanguages = function () {
      return u(N);
    }),
    (a.getLanguage = B),
    (a.autoDetection = M),
    (a.inherit = g),
    (a.IR = a.IDENT_RE = "[a-zA-Z]\\w*"),
    (a.UIR = a.UNDERSCORE_IDENT_RE = "[a-zA-Z_]\\w*"),
    (a.NR = a.NUMBER_RE = "\\b\\d+(\\.\\d+)?"),
    (a.CNR = a.C_NUMBER_RE =
      "(-?)(\\b0[xX][a-fA-F0-9]+|(\\b\\d+(\\.\\d*)?|\\.\\d+)([eE][-+]?\\d+)?)"),
    (a.BNR = a.BINARY_NUMBER_RE = "\\b(0b[01]+)"),
    (a.RSR = a.RE_STARTERS_RE =
      "!|!=|!==|%|%=|&|&&|&=|\\*|\\*=|\\+|\\+=|,|-|-=|/=|/|:|;|<<|<<=|<=|<|===|==|=|>>>=|>>=|>=|>>>|>>|>|\\?|\\[|\\{|\\(|\\^|\\^=|\\||\\|=|\\|\\||~"),
    (a.BE = a.BACKSLASH_ESCAPE = { b: "\\\\[\\s\\S]", r: 0 }),
    (a.ASM = a.APOS_STRING_MODE =
      { cN: "string", b: "'", e: "'", i: "\\n", c: [a.BE] }),
    (a.QSM = a.QUOTE_STRING_MODE =
      { cN: "string", b: '"', e: '"', i: "\\n", c: [a.BE] }),
    (a.PWM = a.PHRASAL_WORDS_MODE =
      {
        b: /\b(a|an|the|are|I'm|isn't|don't|doesn't|won't|but|just|should|pretty|simply|enough|gonna|going|wtf|so|such|will|you|your|they|like|more)\b/,
      }),
    (a.C = a.COMMENT =
      function (e, n, t) {
        var r = a.inherit({ cN: "comment", b: e, e: n, c: [] }, t || {});
        return (
          r.c.push(a.PWM),
          r.c.push({ cN: "doctag", b: "(?:TODO|FIXME|NOTE|BUG|XXX):", r: 0 }),
          r
        );
      }),
    (a.CLCM = a.C_LINE_COMMENT_MODE = a.C("//", "$")),
    (a.CBCM = a.C_BLOCK_COMMENT_MODE = a.C("/\\*", "\\*/")),
    (a.HCM = a.HASH_COMMENT_MODE = a.C("#", "$")),
    (a.NM = a.NUMBER_MODE = { cN: "number", b: a.NR, r: 0 }),
    (a.CNM = a.C_NUMBER_MODE = { cN: "number", b: a.CNR, r: 0 }),
    (a.BNM = a.BINARY_NUMBER_MODE = { cN: "number", b: a.BNR, r: 0 }),
    (a.CSSNM = a.CSS_NUMBER_MODE =
      {
        cN: "number",
        b:
          a.NR +
          "(%|em|ex|ch|rem|vw|vh|vmin|vmax|cm|mm|in|pt|pc|px|deg|grad|rad|turn|s|ms|Hz|kHz|dpi|dpcm|dppx)?",
        r: 0,
      }),
    (a.RM = a.REGEXP_MODE =
      {
        cN: "regexp",
        b: /\//,
        e: /\/[gimuy]*/,
        i: /\n/,
        c: [a.BE, { b: /\[/, e: /\]/, r: 0, c: [a.BE] }],
      }),
    (a.TM = a.TITLE_MODE = { cN: "title", b: a.IR, r: 0 }),
    (a.UTM = a.UNDERSCORE_TITLE_MODE = { cN: "title", b: a.UIR, r: 0 }),
    (a.METHOD_GUARD = { b: "\\.\\s*" + a.UIR, r: 0 }),
    a
  );
});
hljs.registerLanguage("properties", function (r) {
  var t = "[ \\t\\f]*",
    e = "(" + t + "[:=]" + t + "|[ \\t\\f]+)",
    s = "([^\\\\\\W:= \\t\\f\\n]|\\\\.)+",
    n = "([^\\\\:= \\t\\f\\n]|\\\\.)+",
    a = {
      e: e,
      r: 0,
      starts: { cN: "string", e: /$/, r: 0, c: [{ b: "\\\\\\n" }] },
    };
  return {
    cI: !0,
    i: /\S/,
    c: [
      r.C("^\\s*[!#]", "$"),
      {
        b: s + e,
        rB: !0,
        c: [{ cN: "attr", b: s, endsParent: !0, r: 0 }],
        starts: a,
      },
      {
        b: n + e,
        rB: !0,
        r: 0,
        c: [{ cN: "meta", b: n, endsParent: !0, r: 0 }],
        starts: a,
      },
      { cN: "attr", r: 0, b: n + t + "$" },
    ],
  };
});
hljs.registerLanguage("python", function (e) {
  var r = {
      keyword:
        "and elif is global as in if from raise for except finally print import pass return exec else break not with class assert yield try while continue del or def lambda async await nonlocal|10",
      built_in: "Ellipsis NotImplemented",
      literal: "False None True",
    },
    b = { cN: "meta", b: /^(>>>|\.\.\.) / },
    c = { cN: "subst", b: /\{/, e: /\}/, k: r, i: /#/ },
    a = {
      cN: "string",
      c: [e.BE],
      v: [
        { b: /(u|b)?r?'''/, e: /'''/, c: [e.BE, b], r: 10 },
        { b: /(u|b)?r?"""/, e: /"""/, c: [e.BE, b], r: 10 },
        { b: /(fr|rf|f)'''/, e: /'''/, c: [e.BE, b, c] },
        { b: /(fr|rf|f)"""/, e: /"""/, c: [e.BE, b, c] },
        { b: /(u|r|ur)'/, e: /'/, r: 10 },
        { b: /(u|r|ur)"/, e: /"/, r: 10 },
        { b: /(b|br)'/, e: /'/ },
        { b: /(b|br)"/, e: /"/ },
        { b: /(fr|rf|f)'/, e: /'/, c: [e.BE, c] },
        { b: /(fr|rf|f)"/, e: /"/, c: [e.BE, c] },
        e.ASM,
        e.QSM,
      ],
    },
    i = {
      cN: "number",
      r: 0,
      v: [
        { b: e.BNR + "[lLjJ]?" },
        { b: "\\b(0o[0-7]+)[lLjJ]?" },
        { b: e.CNR + "[lLjJ]?" },
      ],
    },
    l = { cN: "params", b: /\(/, e: /\)/, c: ["self", b, i, a] };
  return (
    (c.c = [a, i, b]),
    {
      aliases: ["py", "gyp", "ipython"],
      k: r,
      i: /(<\/|->|\?)|=>/,
      c: [
        b,
        i,
        a,
        e.HCM,
        {
          v: [
            { cN: "function", bK: "def" },
            { cN: "class", bK: "class" },
          ],
          e: /:/,
          i: /[${=;\n,]/,
          c: [e.UTM, l, { b: /->/, eW: !0, k: "None" }],
        },
        { cN: "meta", b: /^[\t ]*@/, e: /$/ },
        { b: /\b(print|exec)\(/ },
      ],
    }
  );
});
hljs.registerLanguage("rust", function (e) {
  var t = "([ui](8|16|32|64|128|size)|f(32|64))?",
    r =
      "drop i8 i16 i32 i64 i128 isize u8 u16 u32 u64 u128 usize f32 f64 str char bool Box Option Result String Vec Copy Send Sized Sync Drop Fn FnMut FnOnce ToOwned Clone Debug PartialEq PartialOrd Eq Ord AsRef AsMut Into From Default Iterator Extend IntoIterator DoubleEndedIterator ExactSizeIterator SliceConcatExt ToString assert! assert_eq! bitflags! bytes! cfg! col! concat! concat_idents! debug_assert! debug_assert_eq! env! panic! file! format! format_args! include_bin! include_str! line! local_data_key! module_path! option_env! print! println! select! stringify! try! unimplemented! unreachable! vec! write! writeln! macro_rules! assert_ne! debug_assert_ne!";
  return {
    aliases: ["rs"],
    k: {
      keyword:
        "abstract as async await become box break const continue crate do dyn else enum extern false final fn for if impl in let loop macro match mod move mut override priv pub ref return self Self static struct super trait true try type typeof unsafe unsized use virtual where while yield",
      literal: "true false Some None Ok Err",
      built_in: r,
    },
    l: e.IR + "!?",
    i: "</",
    c: [
      e.CLCM,
      e.C("/\\*", "\\*/", { c: ["self"] }),
      e.inherit(e.QSM, { b: /b?"/, i: null }),
      {
        cN: "string",
        v: [
          { b: /r(#*)"(.|\n)*?"\1(?!#)/ },
          { b: /b?'\\?(x\w{2}|u\w{4}|U\w{8}|.)'/ },
        ],
      },
      { cN: "symbol", b: /'[a-zA-Z_][a-zA-Z0-9_]*/ },
      {
        cN: "number",
        v: [
          { b: "\\b0b([01_]+)" + t },
          { b: "\\b0o([0-7_]+)" + t },
          { b: "\\b0x([A-Fa-f0-9_]+)" + t },
          { b: "\\b(\\d[\\d_]*(\\.[0-9_]+)?([eE][+-]?[0-9_]+)?)" + t },
        ],
        r: 0,
      },
      { cN: "function", bK: "fn", e: "(\\(|<)", eE: !0, c: [e.UTM] },
      {
        cN: "meta",
        b: "#\\!?\\[",
        e: "\\]",
        c: [{ cN: "meta-string", b: /"/, e: /"/ }],
      },
      {
        cN: "class",
        bK: "type",
        e: ";",
        c: [e.inherit(e.UTM, { endsParent: !0 })],
        i: "\\S",
      },
      {
        cN: "class",
        bK: "trait enum struct union",
        e: "{",
        c: [e.inherit(e.UTM, { endsParent: !0 })],
        i: "[\\w\\d]",
      },
      { b: e.IR + "::", k: { built_in: r } },
      { b: "->" },
    ],
  };
});
hljs.registerLanguage("less", function (e) {
  function r(e) {
    return { cN: "string", b: "~?" + e + ".*?" + e };
  }
  function t(e, r, t) {
    return { cN: e, b: r, r: t };
  }
  var a = "[\\w-]+",
    c = "(" + a + "|@{" + a + "})",
    s = [],
    b = [],
    n = { b: "\\(", e: "\\)", c: b, r: 0 };
  b.push(
    e.CLCM,
    e.CBCM,
    r("'"),
    r('"'),
    e.CSSNM,
    { b: "(url|data-uri)\\(", starts: { cN: "string", e: "[\\)\\n]", eE: !0 } },
    t("number", "#[0-9A-Fa-f]+\\b"),
    n,
    t("variable", "@@?" + a, 10),
    t("variable", "@{" + a + "}"),
    t("built_in", "~?`[^`]*?`"),
    { cN: "attribute", b: a + "\\s*:", e: ":", rB: !0, eE: !0 },
    { cN: "meta", b: "!important" }
  );
  var i = b.concat({ b: "{", e: "}", c: s }),
    o = { bK: "when", eW: !0, c: [{ bK: "and not" }].concat(b) },
    u = {
      b: c + "\\s*:",
      rB: !0,
      e: "[;}]",
      r: 0,
      c: [
        {
          cN: "attribute",
          b: c,
          e: ":",
          eE: !0,
          starts: { eW: !0, i: "[<=$]", r: 0, c: b },
        },
      ],
    },
    l = {
      cN: "keyword",
      b: "@(import|media|charset|font-face|(-[a-z]+-)?keyframes|supports|document|namespace|page|viewport|host)\\b",
      starts: { e: "[;{}]", rE: !0, c: b, r: 0 },
    },
    C = {
      cN: "variable",
      v: [{ b: "@" + a + "\\s*:", r: 15 }, { b: "@" + a }],
      starts: { e: "[;}]", rE: !0, c: i },
    },
    p = {
      v: [
        { b: "[\\.#:&\\[>]", e: "[;{}]" },
        { b: c, e: "{" },
      ],
      rB: !0,
      rE: !0,
      i: "[<='$\"]",
      r: 0,
      c: [
        e.CLCM,
        e.CBCM,
        o,
        t("keyword", "all\\b"),
        t("variable", "@{" + a + "}"),
        t("selector-tag", c + "%?", 0),
        t("selector-id", "#" + c),
        t("selector-class", "\\." + c, 0),
        t("selector-tag", "&", 0),
        { cN: "selector-attr", b: "\\[", e: "\\]" },
        { cN: "selector-pseudo", b: /:(:)?[a-zA-Z0-9\_\-\+\(\)"'.]+/ },
        { b: "\\(", e: "\\)", c: i },
        { b: "!important" },
      ],
    };
  return s.push(e.CLCM, e.CBCM, l, C, u, p), { cI: !0, i: "[=>'/<($\"]", c: s };
});
hljs.registerLanguage("perl", function (e) {
  var t =
      "getpwent getservent quotemeta msgrcv scalar kill dbmclose undef lc ma syswrite tr send umask sysopen shmwrite vec qx utime local oct semctl localtime readpipe do return format read sprintf dbmopen pop getpgrp not getpwnam rewinddir qqfileno qw endprotoent wait sethostent bless s|0 opendir continue each sleep endgrent shutdown dump chomp connect getsockname die socketpair close flock exists index shmgetsub for endpwent redo lstat msgctl setpgrp abs exit select print ref gethostbyaddr unshift fcntl syscall goto getnetbyaddr join gmtime symlink semget splice x|0 getpeername recv log setsockopt cos last reverse gethostbyname getgrnam study formline endhostent times chop length gethostent getnetent pack getprotoent getservbyname rand mkdir pos chmod y|0 substr endnetent printf next open msgsnd readdir use unlink getsockopt getpriority rindex wantarray hex system getservbyport endservent int chr untie rmdir prototype tell listen fork shmread ucfirst setprotoent else sysseek link getgrgid shmctl waitpid unpack getnetbyname reset chdir grep split require caller lcfirst until warn while values shift telldir getpwuid my getprotobynumber delete and sort uc defined srand accept package seekdir getprotobyname semop our rename seek if q|0 chroot sysread setpwent no crypt getc chown sqrt write setnetent setpriority foreach tie sin msgget map stat getlogin unless elsif truncate exec keys glob tied closedirioctl socket readlink eval xor readline binmode setservent eof ord bind alarm pipe atan2 getgrent exp time push setgrent gt lt or ne m|0 break given say state when",
    r = { cN: "subst", b: "[$@]\\{", e: "\\}", k: t },
    s = { b: "->{", e: "}" },
    n = {
      v: [
        { b: /\$\d/ },
        { b: /[\$%@](\^\w\b|#\w+(::\w+)*|{\w+}|\w+(::\w*)*)/ },
        { b: /[\$%@][^\s\w{]/, r: 0 },
      ],
    },
    i = [e.BE, r, n],
    o = [
      n,
      e.HCM,
      e.C("^\\=\\w", "\\=cut", { eW: !0 }),
      s,
      {
        cN: "string",
        c: i,
        v: [
          { b: "q[qwxr]?\\s*\\(", e: "\\)", r: 5 },
          { b: "q[qwxr]?\\s*\\[", e: "\\]", r: 5 },
          { b: "q[qwxr]?\\s*\\{", e: "\\}", r: 5 },
          { b: "q[qwxr]?\\s*\\|", e: "\\|", r: 5 },
          { b: "q[qwxr]?\\s*\\<", e: "\\>", r: 5 },
          { b: "qw\\s+q", e: "q", r: 5 },
          { b: "'", e: "'", c: [e.BE] },
          { b: '"', e: '"' },
          { b: "`", e: "`", c: [e.BE] },
          { b: "{\\w+}", c: [], r: 0 },
          { b: "-?\\w+\\s*\\=\\>", c: [], r: 0 },
        ],
      },
      {
        cN: "number",
        b: "(\\b0[0-7_]+)|(\\b0x[0-9a-fA-F_]+)|(\\b[1-9][0-9_]*(\\.[0-9_]+)?)|[0_]\\b",
        r: 0,
      },
      {
        b: "(\\/\\/|" + e.RSR + "|\\b(split|return|print|reverse|grep)\\b)\\s*",
        k: "split return print reverse grep",
        r: 0,
        c: [
          e.HCM,
          {
            cN: "regexp",
            b: "(s|tr|y)/(\\\\.|[^/])*/(\\\\.|[^/])*/[a-z]*",
            r: 10,
          },
          { cN: "regexp", b: "(m|qr)?/", e: "/[a-z]*", c: [e.BE], r: 0 },
        ],
      },
      {
        cN: "function",
        bK: "sub",
        e: "(\\s*\\(.*?\\))?[;{]",
        eE: !0,
        r: 5,
        c: [e.TM],
      },
      { b: "-\\w\\b", r: 0 },
      {
        b: "^__DATA__$",
        e: "^__END__$",
        sL: "mojolicious",
        c: [{ b: "^@@.*", e: "$", cN: "comment" }],
      },
    ];
  return (r.c = o), { aliases: ["pl", "pm"], l: /[\w\.]+/, k: t, c: (s.c = o) };
});
hljs.registerLanguage("diff", function (e) {
  return {
    aliases: ["patch"],
    c: [
      {
        cN: "meta",
        r: 10,
        v: [
          { b: /^@@ +\-\d+,\d+ +\+\d+,\d+ +@@$/ },
          { b: /^\*\*\* +\d+,\d+ +\*\*\*\*$/ },
          { b: /^\-\-\- +\d+,\d+ +\-\-\-\-$/ },
        ],
      },
      {
        cN: "comment",
        v: [
          { b: /Index: /, e: /$/ },
          { b: /={3,}/, e: /$/ },
          { b: /^\-{3}/, e: /$/ },
          { b: /^\*{3} /, e: /$/ },
          { b: /^\+{3}/, e: /$/ },
          { b: /\*{5}/, e: /\*{5}$/ },
        ],
      },
      { cN: "addition", b: "^\\+", e: "$" },
      { cN: "deletion", b: "^\\-", e: "$" },
      { cN: "addition", b: "^\\!", e: "$" },
    ],
  };
});
hljs.registerLanguage("scss", function (e) {
  var t = { cN: "variable", b: "(\\$[a-zA-Z-][a-zA-Z0-9_-]*)\\b" },
    i = { cN: "number", b: "#[0-9A-Fa-f]+" };
  e.CSSNM, e.QSM, e.ASM, e.CBCM;
  return {
    cI: !0,
    i: "[=/|']",
    c: [
      e.CLCM,
      e.CBCM,
      { cN: "selector-id", b: "\\#[A-Za-z0-9_-]+", r: 0 },
      { cN: "selector-class", b: "\\.[A-Za-z0-9_-]+", r: 0 },
      { cN: "selector-attr", b: "\\[", e: "\\]", i: "$" },
      {
        cN: "selector-tag",
        b: "\\b(a|abbr|acronym|address|area|article|aside|audio|b|base|big|blockquote|body|br|button|canvas|caption|cite|code|col|colgroup|command|datalist|dd|del|details|dfn|div|dl|dt|em|embed|fieldset|figcaption|figure|footer|form|frame|frameset|(h[1-6])|head|header|hgroup|hr|html|i|iframe|img|input|ins|kbd|keygen|label|legend|li|link|map|mark|meta|meter|nav|noframes|noscript|object|ol|optgroup|option|output|p|param|pre|progress|q|rp|rt|ruby|samp|script|section|select|small|span|strike|strong|style|sub|sup|table|tbody|td|textarea|tfoot|th|thead|time|title|tr|tt|ul|var|video)\\b",
        r: 0,
      },
      {
        b: ":(visited|valid|root|right|required|read-write|read-only|out-range|optional|only-of-type|only-child|nth-of-type|nth-last-of-type|nth-last-child|nth-child|not|link|left|last-of-type|last-child|lang|invalid|indeterminate|in-range|hover|focus|first-of-type|first-line|first-letter|first-child|first|enabled|empty|disabled|default|checked|before|after|active)",
      },
      {
        b: "::(after|before|choices|first-letter|first-line|repeat-index|repeat-item|selection|value)",
      },
      t,
      {
        cN: "attribute",
        b: "\\b(z-index|word-wrap|word-spacing|word-break|width|widows|white-space|visibility|vertical-align|unicode-bidi|transition-timing-function|transition-property|transition-duration|transition-delay|transition|transform-style|transform-origin|transform|top|text-underline-position|text-transform|text-shadow|text-rendering|text-overflow|text-indent|text-decoration-style|text-decoration-line|text-decoration-color|text-decoration|text-align-last|text-align|tab-size|table-layout|right|resize|quotes|position|pointer-events|perspective-origin|perspective|page-break-inside|page-break-before|page-break-after|padding-top|padding-right|padding-left|padding-bottom|padding|overflow-y|overflow-x|overflow-wrap|overflow|outline-width|outline-style|outline-offset|outline-color|outline|orphans|order|opacity|object-position|object-fit|normal|none|nav-up|nav-right|nav-left|nav-index|nav-down|min-width|min-height|max-width|max-height|mask|marks|margin-top|margin-right|margin-left|margin-bottom|margin|list-style-type|list-style-position|list-style-image|list-style|line-height|letter-spacing|left|justify-content|initial|inherit|ime-mode|image-orientation|image-resolution|image-rendering|icon|hyphens|height|font-weight|font-variant-ligatures|font-variant|font-style|font-stretch|font-size-adjust|font-size|font-language-override|font-kerning|font-feature-settings|font-family|font|float|flex-wrap|flex-shrink|flex-grow|flex-flow|flex-direction|flex-basis|flex|filter|empty-cells|display|direction|cursor|counter-reset|counter-increment|content|column-width|column-span|column-rule-width|column-rule-style|column-rule-color|column-rule|column-gap|column-fill|column-count|columns|color|clip-path|clip|clear|caption-side|break-inside|break-before|break-after|box-sizing|box-shadow|box-decoration-break|bottom|border-width|border-top-width|border-top-style|border-top-right-radius|border-top-left-radius|border-top-color|border-top|border-style|border-spacing|border-right-width|border-right-style|border-right-color|border-right|border-radius|border-left-width|border-left-style|border-left-color|border-left|border-image-width|border-image-source|border-image-slice|border-image-repeat|border-image-outset|border-image|border-color|border-collapse|border-bottom-width|border-bottom-style|border-bottom-right-radius|border-bottom-left-radius|border-bottom-color|border-bottom|border|background-size|background-repeat|background-position|background-origin|background-image|background-color|background-clip|background-attachment|background-blend-mode|background|backface-visibility|auto|animation-timing-function|animation-play-state|animation-name|animation-iteration-count|animation-fill-mode|animation-duration|animation-direction|animation-delay|animation|align-self|align-items|align-content)\\b",
        i: "[^\\s]",
      },
      {
        b: "\\b(whitespace|wait|w-resize|visible|vertical-text|vertical-ideographic|uppercase|upper-roman|upper-alpha|underline|transparent|top|thin|thick|text|text-top|text-bottom|tb-rl|table-header-group|table-footer-group|sw-resize|super|strict|static|square|solid|small-caps|separate|se-resize|scroll|s-resize|rtl|row-resize|ridge|right|repeat|repeat-y|repeat-x|relative|progress|pointer|overline|outside|outset|oblique|nowrap|not-allowed|normal|none|nw-resize|no-repeat|no-drop|newspaper|ne-resize|n-resize|move|middle|medium|ltr|lr-tb|lowercase|lower-roman|lower-alpha|loose|list-item|line|line-through|line-edge|lighter|left|keep-all|justify|italic|inter-word|inter-ideograph|inside|inset|inline|inline-block|inherit|inactive|ideograph-space|ideograph-parenthesis|ideograph-numeric|ideograph-alpha|horizontal|hidden|help|hand|groove|fixed|ellipsis|e-resize|double|dotted|distribute|distribute-space|distribute-letter|distribute-all-lines|disc|disabled|default|decimal|dashed|crosshair|collapse|col-resize|circle|char|center|capitalize|break-word|break-all|bottom|both|bolder|bold|block|bidi-override|below|baseline|auto|always|all-scroll|absolute|table|table-cell)\\b",
      },
      {
        b: ":",
        e: ";",
        c: [t, i, e.CSSNM, e.QSM, e.ASM, { cN: "meta", b: "!important" }],
      },
      {
        b: "@",
        e: "[{;]",
        k: "mixin include extend for if else each while charset import debug media page content font-face namespace warn",
        c: [t, e.QSM, e.ASM, i, e.CSSNM, { b: "\\s[A-Za-z0-9_.-]+", r: 0 }],
      },
    ],
  };
});
hljs.registerLanguage("bash", function (e) {
  var t = {
      cN: "variable",
      v: [{ b: /\$[\w\d#@][\w\d_]*/ }, { b: /\$\{(.*?)}/ }],
    },
    s = {
      cN: "string",
      b: /"/,
      e: /"/,
      c: [e.BE, t, { cN: "variable", b: /\$\(/, e: /\)/, c: [e.BE] }],
    };
  return {
    aliases: ["sh", "zsh"],
    l: /\b-?[a-z\._]+\b/,
    k: {
      keyword: "if then else elif fi for while in do done case esac function",
      literal: "true false",
      built_in:
        "break cd continue eval exec exit export getopts hash pwd readonly return shift test times trap umask unset alias bind builtin caller command declare echo enable help let local logout mapfile printf read readarray source type typeset ulimit unalias set shopt autoload bg bindkey bye cap chdir clone comparguments compcall compctl compdescribe compfiles compgroups compquote comptags comptry compvalues dirs disable disown echotc echoti emulate fc fg float functions getcap getln history integer jobs kill limit log noglob popd print pushd pushln rehash sched setcap setopt stat suspend ttyctl unfunction unhash unlimit unsetopt vared wait whence where which zcompile zformat zftp zle zmodload zparseopts zprof zpty zregexparse zsocket zstyle ztcp",
      _: "-ne -eq -lt -gt -f -d -e -s -l -a",
    },
    c: [
      { cN: "meta", b: /^#![^\n]+sh\s*$/, r: 10 },
      {
        cN: "function",
        b: /\w[\w\d_]*\s*\(\s*\)\s*\{/,
        rB: !0,
        c: [e.inherit(e.TM, { b: /\w[\w\d_]*/ })],
        r: 0,
      },
      e.HCM,
      s,
      { cN: "", b: /\\"/ },
      { cN: "string", b: /'/, e: /'/ },
      t,
    ],
  };
});
hljs.registerLanguage("shell", function (s) {
  return {
    aliases: ["console"],
    c: [
      {
        cN: "meta",
        b: "^\\s{0,3}[\\w\\d\\[\\]()@-]*[>%$#]",
        starts: { e: "$", sL: "bash" },
      },
    ],
  };
});
hljs.registerLanguage("makefile", function (e) {
  var i = {
      cN: "variable",
      v: [{ b: "\\$\\(" + e.UIR + "\\)", c: [e.BE] }, { b: /\$[@%<?\^\+\*]/ }],
    },
    r = { cN: "string", b: /"/, e: /"/, c: [e.BE, i] },
    a = {
      cN: "variable",
      b: /\$\([\w-]+\s/,
      e: /\)/,
      k: {
        built_in:
          "subst patsubst strip findstring filter filter-out sort word wordlist firstword lastword dir notdir suffix basename addsuffix addprefix join wildcard realpath abspath error warning shell origin flavor foreach if or and call eval file value",
      },
      c: [i],
    },
    n = {
      b: "^" + e.UIR + "\\s*[:+?]?=",
      i: "\\n",
      rB: !0,
      c: [{ b: "^" + e.UIR, e: "[:+?]?=", eE: !0 }],
    },
    t = { cN: "section", b: /^[^\s]+:/, e: /$/, c: [i] };
  return {
    aliases: ["mk", "mak"],
    k: "define endef undefine ifdef ifndef ifeq ifneq else endif include -include sinclude override export unexport private vpath",
    l: /[\w-]+/,
    c: [
      e.HCM,
      i,
      r,
      a,
      n,
      {
        cN: "meta",
        b: /^\.PHONY:/,
        e: /$/,
        k: { "meta-keyword": ".PHONY" },
        l: /[\.\w]+/,
      },
      t,
    ],
  };
});
hljs.registerLanguage("json", function (e) {
  var i = { literal: "true false null" },
    n = [e.QSM, e.CNM],
    r = { e: ",", eW: !0, eE: !0, c: n, k: i },
    t = {
      b: "{",
      e: "}",
      c: [
        { cN: "attr", b: /"/, e: /"/, c: [e.BE], i: "\\n" },
        e.inherit(r, { b: /:/ }),
      ],
      i: "\\S",
    },
    c = { b: "\\[", e: "\\]", c: [e.inherit(r)], i: "\\S" };
  return n.splice(n.length, 0, t, c), { c: n, k: i, i: "\\S" };
});
hljs.registerLanguage("ini", function (e) {
  var b = {
    cN: "string",
    c: [e.BE],
    v: [
      { b: "'''", e: "'''", r: 10 },
      { b: '"""', e: '"""', r: 10 },
      { b: '"', e: '"' },
      { b: "'", e: "'" },
    ],
  };
  return {
    aliases: ["toml"],
    cI: !0,
    i: /\S/,
    c: [
      e.C(";", "$"),
      e.HCM,
      { cN: "section", b: /^\s*\[+/, e: /\]+/ },
      {
        b: /^[a-z0-9\[\]_\.-]+\s*=\s*/,
        e: "$",
        rB: !0,
        c: [
          { cN: "attr", b: /[a-z0-9\[\]_\.-]+/ },
          {
            b: /=/,
            eW: !0,
            r: 0,
            c: [
              e.C(";", "$"),
              e.HCM,
              { cN: "literal", b: /\bon|off|true|false|yes|no\b/ },
              {
                cN: "variable",
                v: [{ b: /\$[\w\d"][\w\d_]*/ }, { b: /\$\{(.*?)}/ }],
              },
              b,
              { cN: "number", b: /([\+\-]+)?[\d]+_[\d_]+/ },
              e.NM,
            ],
          },
        ],
      },
    ],
  };
});
hljs.registerLanguage("http", function (e) {
  var t = "HTTP/[0-9\\.]+";
  return {
    aliases: ["https"],
    i: "\\S",
    c: [
      { b: "^" + t, e: "$", c: [{ cN: "number", b: "\\b\\d{3}\\b" }] },
      {
        b: "^[A-Z]+ (.*?) " + t + "$",
        rB: !0,
        e: "$",
        c: [
          { cN: "string", b: " ", e: " ", eB: !0, eE: !0 },
          { b: t },
          { cN: "keyword", b: "[A-Z]+" },
        ],
      },
      {
        cN: "attribute",
        b: "^\\w",
        e: ": ",
        eE: !0,
        i: "\\n|\\s|=",
        starts: { e: "$", r: 0 },
      },
      { b: "\\n\\n", starts: { sL: [], eW: !0 } },
    ],
  };
});
hljs.registerLanguage("coffeescript", function (e) {
  var c = {
      keyword:
        "in if for while finally new do return else break catch instanceof throw try this switch continue typeof delete debugger super yield import export from as default await then unless until loop of by when and or is isnt not",
      literal: "true false null undefined yes no on off",
      built_in: "npm require console print module global window document",
    },
    n = "[A-Za-z$_][0-9A-Za-z$_]*",
    r = { cN: "subst", b: /#\{/, e: /}/, k: c },
    i = [
      e.BNM,
      e.inherit(e.CNM, { starts: { e: "(\\s*/)?", r: 0 } }),
      {
        cN: "string",
        v: [
          { b: /'''/, e: /'''/, c: [e.BE] },
          { b: /'/, e: /'/, c: [e.BE] },
          { b: /"""/, e: /"""/, c: [e.BE, r] },
          { b: /"/, e: /"/, c: [e.BE, r] },
        ],
      },
      {
        cN: "regexp",
        v: [
          { b: "///", e: "///", c: [r, e.HCM] },
          { b: "//[gim]*", r: 0 },
          { b: /\/(?![ *])(\\\/|.)*?\/[gim]*(?=\W|$)/ },
        ],
      },
      { b: "@" + n },
      {
        sL: "javascript",
        eB: !0,
        eE: !0,
        v: [
          { b: "```", e: "```" },
          { b: "`", e: "`" },
        ],
      },
    ];
  r.c = i;
  var s = e.inherit(e.TM, { b: n }),
    t = "(\\(.*\\))?\\s*\\B[-=]>",
    o = {
      cN: "params",
      b: "\\([^\\(]",
      rB: !0,
      c: [{ b: /\(/, e: /\)/, k: c, c: ["self"].concat(i) }],
    };
  return {
    aliases: ["coffee", "cson", "iced"],
    k: c,
    i: /\/\*/,
    c: i.concat([
      e.C("###", "###"),
      e.HCM,
      {
        cN: "function",
        b: "^\\s*" + n + "\\s*=\\s*" + t,
        e: "[-=]>",
        rB: !0,
        c: [s, o],
      },
      {
        b: /[:\(,=]\s*/,
        r: 0,
        c: [{ cN: "function", b: t, e: "[-=]>", rB: !0, c: [o] }],
      },
      {
        cN: "class",
        bK: "class",
        e: "$",
        i: /[:="\[\]]/,
        c: [{ bK: "extends", eW: !0, i: /[:="\[\]]/, c: [s] }, s],
      },
      { b: n + ":", e: ":", rB: !0, rE: !0, r: 0 },
    ]),
  };
});
hljs.registerLanguage("css", function (e) {
  var c = {
    b: /(?:[A-Z\_\.\-]+|--[a-zA-Z0-9_-]+)\s*:/,
    rB: !0,
    e: ";",
    eW: !0,
    c: [
      {
        cN: "attribute",
        b: /\S/,
        e: ":",
        eE: !0,
        starts: {
          eW: !0,
          eE: !0,
          c: [
            {
              b: /[\w-]+\(/,
              rB: !0,
              c: [
                { cN: "built_in", b: /[\w-]+/ },
                { b: /\(/, e: /\)/, c: [e.ASM, e.QSM] },
              ],
            },
            e.CSSNM,
            e.QSM,
            e.ASM,
            e.CBCM,
            { cN: "number", b: "#[0-9A-Fa-f]+" },
            { cN: "meta", b: "!important" },
          ],
        },
      },
    ],
  };
  return {
    cI: !0,
    i: /[=\/|'\$]/,
    c: [
      e.CBCM,
      { cN: "selector-id", b: /#[A-Za-z0-9_-]+/ },
      { cN: "selector-class", b: /\.[A-Za-z0-9_-]+/ },
      { cN: "selector-attr", b: /\[/, e: /\]/, i: "$" },
      { cN: "selector-pseudo", b: /:(:)?[a-zA-Z0-9\_\-\+\(\)"'.]+/ },
      { b: "@(font-face|page)", l: "[a-z-]+", k: "font-face page" },
      {
        b: "@",
        e: "[{;]",
        i: /:/,
        c: [
          { cN: "keyword", b: /\w+/ },
          { b: /\s/, eW: !0, eE: !0, r: 0, c: [e.ASM, e.QSM, e.CSSNM] },
        ],
      },
      { cN: "selector-tag", b: "[a-zA-Z-][a-zA-Z0-9_-]*", r: 0 },
      { b: "{", e: "}", i: /\S/, c: [e.CBCM, c] },
    ],
  };
});
hljs.registerLanguage("objectivec", function (e) {
  var t = /[a-zA-Z@][a-zA-Z0-9_]*/,
    _ = "@interface @class @protocol @implementation";
  return {
    aliases: ["mm", "objc", "obj-c"],
    k: {
      keyword:
        "int float while char export sizeof typedef const struct for union unsigned long volatile static bool mutable if do return goto void enum else break extern asm case short default double register explicit signed typename this switch continue wchar_t inline readonly assign readwrite self @synchronized id typeof nonatomic super unichar IBOutlet IBAction strong weak copy in out inout bycopy byref oneway __strong __weak __block __autoreleasing @private @protected @public @try @property @end @throw @catch @finally @autoreleasepool @synthesize @dynamic @selector @optional @required @encode @package @import @defs @compatibility_alias __bridge __bridge_transfer __bridge_retained __bridge_retain __covariant __contravariant __kindof _Nonnull _Nullable _Null_unspecified __FUNCTION__ __PRETTY_FUNCTION__ __attribute__ getter setter retain unsafe_unretained nonnull nullable null_unspecified null_resettable class instancetype NS_DESIGNATED_INITIALIZER NS_UNAVAILABLE NS_REQUIRES_SUPER NS_RETURNS_INNER_POINTER NS_INLINE NS_AVAILABLE NS_DEPRECATED NS_ENUM NS_OPTIONS NS_SWIFT_UNAVAILABLE NS_ASSUME_NONNULL_BEGIN NS_ASSUME_NONNULL_END NS_REFINED_FOR_SWIFT NS_SWIFT_NAME NS_SWIFT_NOTHROW NS_DURING NS_HANDLER NS_ENDHANDLER NS_VALUERETURN NS_VOIDRETURN",
      literal: "false true FALSE TRUE nil YES NO NULL",
      built_in:
        "BOOL dispatch_once_t dispatch_queue_t dispatch_sync dispatch_async dispatch_once",
    },
    l: t,
    i: "</",
    c: [
      {
        cN: "built_in",
        b: "\\b(AV|CA|CF|CG|CI|CL|CM|CN|CT|MK|MP|MTK|MTL|NS|SCN|SK|UI|WK|XC)\\w+",
      },
      e.CLCM,
      e.CBCM,
      e.CNM,
      e.QSM,
      {
        cN: "string",
        v: [
          { b: '@"', e: '"', i: "\\n", c: [e.BE] },
          { b: "'", e: "[^\\\\]'", i: "[^\\\\][^']" },
        ],
      },
      {
        cN: "meta",
        b: "#",
        e: "$",
        c: [
          {
            cN: "meta-string",
            v: [
              { b: '"', e: '"' },
              { b: "<", e: ">" },
            ],
          },
        ],
      },
      {
        cN: "class",
        b: "(" + _.split(" ").join("|") + ")\\b",
        e: "({|$)",
        eE: !0,
        k: _,
        l: t,
        c: [e.UTM],
      },
      { b: "\\." + e.UIR, r: 0 },
    ],
  };
});
hljs.registerLanguage("ruby", function (e) {
  var b =
      "[a-zA-Z_]\\w*[!?=]?|[-+~]\\@|<<|>>|=~|===?|<=>|[<>]=?|\\*\\*|[-/+%^&*~`|]|\\[\\]=?",
    r = {
      keyword:
        "and then defined module in return redo if BEGIN retry end for self when next until do begin unless END rescue else break undef not super class case require yield alias while ensure elsif or include attr_reader attr_writer attr_accessor",
      literal: "true false nil",
    },
    c = { cN: "doctag", b: "@[A-Za-z]+" },
    a = { b: "#<", e: ">" },
    s = [
      e.C("#", "$", { c: [c] }),
      e.C("^\\=begin", "^\\=end", { c: [c], r: 10 }),
      e.C("^__END__", "\\n$"),
    ],
    n = { cN: "subst", b: "#\\{", e: "}", k: r },
    t = {
      cN: "string",
      c: [e.BE, n],
      v: [
        { b: /'/, e: /'/ },
        { b: /"/, e: /"/ },
        { b: /`/, e: /`/ },
        { b: "%[qQwWx]?\\(", e: "\\)" },
        { b: "%[qQwWx]?\\[", e: "\\]" },
        { b: "%[qQwWx]?{", e: "}" },
        { b: "%[qQwWx]?<", e: ">" },
        { b: "%[qQwWx]?/", e: "/" },
        { b: "%[qQwWx]?%", e: "%" },
        { b: "%[qQwWx]?-", e: "-" },
        { b: "%[qQwWx]?\\|", e: "\\|" },
        { b: /\B\?(\\\d{1,3}|\\x[A-Fa-f0-9]{1,2}|\\u[A-Fa-f0-9]{4}|\\?\S)\b/ },
        {
          b: /<<[-~]?'?(\w+)(?:.|\n)*?\n\s*\1\b/,
          rB: !0,
          c: [
            { b: /<<[-~]?'?/ },
            { b: /\w+/, endSameAsBegin: !0, c: [e.BE, n] },
          ],
        },
      ],
    },
    i = { cN: "params", b: "\\(", e: "\\)", endsParent: !0, k: r },
    d = [
      t,
      a,
      {
        cN: "class",
        bK: "class module",
        e: "$|;",
        i: /=/,
        c: [
          e.inherit(e.TM, { b: "[A-Za-z_]\\w*(::\\w+)*(\\?|\\!)?" }),
          { b: "<\\s*", c: [{ b: "(" + e.IR + "::)?" + e.IR }] },
        ].concat(s),
      },
      {
        cN: "function",
        bK: "def",
        e: "$|;",
        c: [e.inherit(e.TM, { b: b }), i].concat(s),
      },
      { b: e.IR + "::" },
      { cN: "symbol", b: e.UIR + "(\\!|\\?)?:", r: 0 },
      { cN: "symbol", b: ":(?!\\s)", c: [t, { b: b }], r: 0 },
      {
        cN: "number",
        b: "(\\b0[0-7_]+)|(\\b0x[0-9a-fA-F_]+)|(\\b[1-9][0-9_]*(\\.[0-9_]+)?)|[0_]\\b",
        r: 0,
      },
      { b: "(\\$\\W)|((\\$|\\@\\@?)(\\w+))" },
      { cN: "params", b: /\|/, e: /\|/, k: r },
      {
        b: "(" + e.RSR + "|unless)\\s*",
        k: "unless",
        c: [
          a,
          {
            cN: "regexp",
            c: [e.BE, n],
            i: /\n/,
            v: [
              { b: "/", e: "/[a-z]*" },
              { b: "%r{", e: "}[a-z]*" },
              { b: "%r\\(", e: "\\)[a-z]*" },
              { b: "%r!", e: "![a-z]*" },
              { b: "%r\\[", e: "\\][a-z]*" },
            ],
          },
        ].concat(s),
        r: 0,
      },
    ].concat(s);
  n.c = d;
  var l = [
    { b: /^\s*=>/, starts: { e: "$", c: (i.c = d) } },
    {
      cN: "meta",
      b: "^([>?]>|[\\w#]+\\(\\w+\\):\\d+:\\d+>|(\\w+-)?\\d+\\.\\d+\\.\\d(p\\d+)?[^>]+>)",
      starts: { e: "$", c: d },
    },
  ];
  return {
    aliases: ["rb", "gemspec", "podspec", "thor", "irb"],
    k: r,
    i: /\/\*/,
    c: s.concat(l).concat(d),
  };
});
hljs.registerLanguage("yaml", function (e) {
  var b = "true false yes no null",
    a = "^[ \\-]*",
    r = "[a-zA-Z_][\\w\\-]*",
    t = {
      cN: "attr",
      v: [
        { b: a + r + ":" },
        { b: a + '"' + r + '":' },
        { b: a + "'" + r + "':" },
      ],
    },
    c = {
      cN: "string",
      r: 0,
      v: [{ b: /'/, e: /'/ }, { b: /"/, e: /"/ }, { b: /\S+/ }],
      c: [
        e.BE,
        {
          cN: "template-variable",
          v: [
            { b: "{{", e: "}}" },
            { b: "%{", e: "}" },
          ],
        },
      ],
    };
  return {
    cI: !0,
    aliases: ["yml", "YAML", "yaml"],
    c: [
      t,
      { cN: "meta", b: "^---s*$", r: 10 },
      { cN: "string", b: "[\\|>] *$", rE: !0, c: c.c, e: t.v[0].b },
      { b: "<%[%=-]?", e: "[%-]?%>", sL: "ruby", eB: !0, eE: !0, r: 0 },
      { cN: "type", b: "!" + e.UIR },
      { cN: "type", b: "!!" + e.UIR },
      { cN: "meta", b: "&" + e.UIR + "$" },
      { cN: "meta", b: "\\*" + e.UIR + "$" },
      { cN: "bullet", b: "^ *-", r: 0 },
      e.HCM,
      { bK: b, k: { literal: b } },
      e.CNM,
      c,
    ],
  };
});
hljs.registerLanguage("java", function (e) {
  var a =
      "false synchronized int abstract float private char boolean var static null if const for true while long strictfp finally protected import native final void enum else break transient catch instanceof byte super volatile case assert short package default double public try this switch continue throws protected public private module requires exports do",
    t = {
      cN: "number",
      b: "\\b(0[bB]([01]+[01_]+[01]+|[01]+)|0[xX]([a-fA-F0-9]+[a-fA-F0-9_]+[a-fA-F0-9]+|[a-fA-F0-9]+)|(([\\d]+[\\d_]+[\\d]+|[\\d]+)(\\.([\\d]+[\\d_]+[\\d]+|[\\d]+))?|\\.([\\d]+[\\d_]+[\\d]+|[\\d]+))([eE][-+]?\\d+)?)[lLfF]?",
      r: 0,
    };
  return {
    aliases: ["jsp"],
    k: a,
    i: /<\/|#/,
    c: [
      e.C("/\\*\\*", "\\*/", {
        r: 0,
        c: [
          { b: /\w+@/, r: 0 },
          { cN: "doctag", b: "@[A-Za-z]+" },
        ],
      }),
      e.CLCM,
      e.CBCM,
      e.ASM,
      e.QSM,
      {
        cN: "class",
        bK: "class interface",
        e: /[{;=]/,
        eE: !0,
        k: "class interface",
        i: /[:"\[\]]/,
        c: [{ bK: "extends implements" }, e.UTM],
      },
      { bK: "new throw return else", r: 0 },
      {
        cN: "function",
        b:
          "([À-ʸa-zA-Z_$][À-ʸa-zA-Z_$0-9]*(<[À-ʸa-zA-Z_$][À-ʸa-zA-Z_$0-9]*(\\s*,\\s*[À-ʸa-zA-Z_$][À-ʸa-zA-Z_$0-9]*)*>)?\\s+)+" +
          e.UIR +
          "\\s*\\(",
        rB: !0,
        e: /[{;=]/,
        eE: !0,
        k: a,
        c: [
          { b: e.UIR + "\\s*\\(", rB: !0, r: 0, c: [e.UTM] },
          {
            cN: "params",
            b: /\(/,
            e: /\)/,
            k: a,
            r: 0,
            c: [e.ASM, e.QSM, e.CNM, e.CBCM],
          },
          e.CLCM,
          e.CBCM,
        ],
      },
      t,
      { cN: "meta", b: "@[A-Za-z]+" },
    ],
  };
});
hljs.registerLanguage("sql", function (e) {
  var t = e.C("--", "$");
  return {
    cI: !0,
    i: /[<>{}*]/,
    c: [
      {
        bK: "begin end start commit rollback savepoint lock alter create drop rename call delete do handler insert load replace select truncate update set show pragma grant merge describe use explain help declare prepare execute deallocate release unlock purge reset change stop analyze cache flush optimize repair kill install uninstall checksum restore check backup revoke comment values with",
        e: /;/,
        eW: !0,
        l: /[\w\.]+/,
        k: {
          keyword:
            "as abort abs absolute acc acce accep accept access accessed accessible account acos action activate add addtime admin administer advanced advise aes_decrypt aes_encrypt after agent aggregate ali alia alias all allocate allow alter always analyze ancillary and anti any anydata anydataset anyschema anytype apply archive archived archivelog are as asc ascii asin assembly assertion associate asynchronous at atan atn2 attr attri attrib attribu attribut attribute attributes audit authenticated authentication authid authors auto autoallocate autodblink autoextend automatic availability avg backup badfile basicfile before begin beginning benchmark between bfile bfile_base big bigfile bin binary_double binary_float binlog bit_and bit_count bit_length bit_or bit_xor bitmap blob_base block blocksize body both bound bucket buffer_cache buffer_pool build bulk by byte byteordermark bytes cache caching call calling cancel capacity cascade cascaded case cast catalog category ceil ceiling chain change changed char_base char_length character_length characters characterset charindex charset charsetform charsetid check checksum checksum_agg child choose chr chunk class cleanup clear client clob clob_base clone close cluster_id cluster_probability cluster_set clustering coalesce coercibility col collate collation collect colu colum column column_value columns columns_updated comment commit compact compatibility compiled complete composite_limit compound compress compute concat concat_ws concurrent confirm conn connec connect connect_by_iscycle connect_by_isleaf connect_by_root connect_time connection consider consistent constant constraint constraints constructor container content contents context contributors controlfile conv convert convert_tz corr corr_k corr_s corresponding corruption cos cost count count_big counted covar_pop covar_samp cpu_per_call cpu_per_session crc32 create creation critical cross cube cume_dist curdate current current_date current_time current_timestamp current_user cursor curtime customdatum cycle data database databases datafile datafiles datalength date_add date_cache date_format date_sub dateadd datediff datefromparts datename datepart datetime2fromparts day day_to_second dayname dayofmonth dayofweek dayofyear days db_role_change dbtimezone ddl deallocate declare decode decompose decrement decrypt deduplicate def defa defau defaul default defaults deferred defi defin define degrees delayed delegate delete delete_all delimited demand dense_rank depth dequeue des_decrypt des_encrypt des_key_file desc descr descri describ describe descriptor deterministic diagnostics difference dimension direct_load directory disable disable_all disallow disassociate discardfile disconnect diskgroup distinct distinctrow distribute distributed div do document domain dotnet double downgrade drop dumpfile duplicate duration each edition editionable editions element ellipsis else elsif elt empty enable enable_all enclosed encode encoding encrypt end end-exec endian enforced engine engines enqueue enterprise entityescaping eomonth error errors escaped evalname evaluate event eventdata events except exception exceptions exchange exclude excluding execu execut execute exempt exists exit exp expire explain explode export export_set extended extent external external_1 external_2 externally extract failed failed_login_attempts failover failure far fast feature_set feature_value fetch field fields file file_name_convert filesystem_like_logging final finish first first_value fixed flash_cache flashback floor flush following follows for forall force foreign form forma format found found_rows freelist freelists freepools fresh from from_base64 from_days ftp full function general generated get get_format get_lock getdate getutcdate global global_name globally go goto grant grants greatest group group_concat group_id grouping grouping_id groups gtid_subtract guarantee guard handler hash hashkeys having hea head headi headin heading heap help hex hierarchy high high_priority hosts hour hours http id ident_current ident_incr ident_seed identified identity idle_time if ifnull ignore iif ilike ilm immediate import in include including increment index indexes indexing indextype indicator indices inet6_aton inet6_ntoa inet_aton inet_ntoa infile initial initialized initially initrans inmemory inner innodb input insert install instance instantiable instr interface interleaved intersect into invalidate invisible is is_free_lock is_ipv4 is_ipv4_compat is_not is_not_null is_used_lock isdate isnull isolation iterate java join json json_exists keep keep_duplicates key keys kill language large last last_day last_insert_id last_value lateral lax lcase lead leading least leaves left len lenght length less level levels library like like2 like4 likec limit lines link list listagg little ln load load_file lob lobs local localtime localtimestamp locate locator lock locked log log10 log2 logfile logfiles logging logical logical_reads_per_call logoff logon logs long loop low low_priority lower lpad lrtrim ltrim main make_set makedate maketime managed management manual map mapping mask master master_pos_wait match matched materialized max maxextents maximize maxinstances maxlen maxlogfiles maxloghistory maxlogmembers maxsize maxtrans md5 measures median medium member memcompress memory merge microsecond mid migration min minextents minimum mining minus minute minutes minvalue missing mod mode model modification modify module monitoring month months mount move movement multiset mutex name name_const names nan national native natural nav nchar nclob nested never new newline next nextval no no_write_to_binlog noarchivelog noaudit nobadfile nocheck nocompress nocopy nocycle nodelay nodiscardfile noentityescaping noguarantee nokeep nologfile nomapping nomaxvalue nominimize nominvalue nomonitoring none noneditionable nonschema noorder nopr nopro noprom nopromp noprompt norely noresetlogs noreverse normal norowdependencies noschemacheck noswitch not nothing notice notnull notrim novalidate now nowait nth_value nullif nulls num numb numbe nvarchar nvarchar2 object ocicoll ocidate ocidatetime ociduration ociinterval ociloblocator ocinumber ociref ocirefcursor ocirowid ocistring ocitype oct octet_length of off offline offset oid oidindex old on online only opaque open operations operator optimal optimize option optionally or oracle oracle_date oradata ord ordaudio orddicom orddoc order ordimage ordinality ordvideo organization orlany orlvary out outer outfile outline output over overflow overriding package pad parallel parallel_enable parameters parent parse partial partition partitions pascal passing password password_grace_time password_lock_time password_reuse_max password_reuse_time password_verify_function patch path patindex pctincrease pctthreshold pctused pctversion percent percent_rank percentile_cont percentile_disc performance period period_add period_diff permanent physical pi pipe pipelined pivot pluggable plugin policy position post_transaction pow power pragma prebuilt precedes preceding precision prediction prediction_cost prediction_details prediction_probability prediction_set prepare present preserve prior priority private private_sga privileges procedural procedure procedure_analyze processlist profiles project prompt protection public publishingservername purge quarter query quick quiesce quota quotename radians raise rand range rank raw read reads readsize rebuild record records recover recovery recursive recycle redo reduced ref reference referenced references referencing refresh regexp_like register regr_avgx regr_avgy regr_count regr_intercept regr_r2 regr_slope regr_sxx regr_sxy reject rekey relational relative relaylog release release_lock relies_on relocate rely rem remainder rename repair repeat replace replicate replication required reset resetlogs resize resource respect restore restricted result result_cache resumable resume retention return returning returns reuse reverse revoke right rlike role roles rollback rolling rollup round row row_count rowdependencies rowid rownum rows rtrim rules safe salt sample save savepoint sb1 sb2 sb4 scan schema schemacheck scn scope scroll sdo_georaster sdo_topo_geometry search sec_to_time second seconds section securefile security seed segment select self semi sequence sequential serializable server servererror session session_user sessions_per_user set sets settings sha sha1 sha2 share shared shared_pool short show shrink shutdown si_averagecolor si_colorhistogram si_featurelist si_positionalcolor si_stillimage si_texture siblings sid sign sin size size_t sizes skip slave sleep smalldatetimefromparts smallfile snapshot some soname sort soundex source space sparse spfile split sql sql_big_result sql_buffer_result sql_cache sql_calc_found_rows sql_small_result sql_variant_property sqlcode sqldata sqlerror sqlname sqlstate sqrt square standalone standby start starting startup statement static statistics stats_binomial_test stats_crosstab stats_ks_test stats_mode stats_mw_test stats_one_way_anova stats_t_test_ stats_t_test_indep stats_t_test_one stats_t_test_paired stats_wsr_test status std stddev stddev_pop stddev_samp stdev stop storage store stored str str_to_date straight_join strcmp strict string struct stuff style subdate subpartition subpartitions substitutable substr substring subtime subtring_index subtype success sum suspend switch switchoffset switchover sync synchronous synonym sys sys_xmlagg sysasm sysaux sysdate sysdatetimeoffset sysdba sysoper system system_user sysutcdatetime table tables tablespace tablesample tan tdo template temporary terminated tertiary_weights test than then thread through tier ties time time_format time_zone timediff timefromparts timeout timestamp timestampadd timestampdiff timezone_abbr timezone_minute timezone_region to to_base64 to_date to_days to_seconds todatetimeoffset trace tracking transaction transactional translate translation treat trigger trigger_nestlevel triggers trim truncate try_cast try_convert try_parse type ub1 ub2 ub4 ucase unarchived unbounded uncompress under undo unhex unicode uniform uninstall union unique unix_timestamp unknown unlimited unlock unnest unpivot unrecoverable unsafe unsigned until untrusted unusable unused update updated upgrade upped upper upsert url urowid usable usage use use_stored_outlines user user_data user_resources users using utc_date utc_timestamp uuid uuid_short validate validate_password_strength validation valist value values var var_samp varcharc vari varia variab variabl variable variables variance varp varraw varrawc varray verify version versions view virtual visible void wait wallet warning warnings week weekday weekofyear wellformed when whene whenev wheneve whenever where while whitespace window with within without work wrapped xdb xml xmlagg xmlattributes xmlcast xmlcolattval xmlelement xmlexists xmlforest xmlindex xmlnamespaces xmlpi xmlquery xmlroot xmlschema xmlserialize xmltable xmltype xor year year_to_month years yearweek",
          literal: "true false null unknown",
          built_in:
            "array bigint binary bit blob bool boolean char character date dec decimal float int int8 integer interval number numeric real record serial serial8 smallint text time timestamp tinyint varchar varying void",
        },
        c: [
          { cN: "string", b: "'", e: "'", c: [e.BE, { b: "''" }] },
          { cN: "string", b: '"', e: '"', c: [e.BE, { b: '""' }] },
          { cN: "string", b: "`", e: "`", c: [e.BE] },
          e.CNM,
          e.CBCM,
          t,
          e.HCM,
        ],
      },
      e.CBCM,
      t,
      e.HCM,
    ],
  };
});
hljs.registerLanguage("apache", function (e) {
  var r = { cN: "number", b: "[\\$%]\\d+" };
  return {
    aliases: ["apacheconf"],
    cI: !0,
    c: [
      e.HCM,
      { cN: "section", b: "</?", e: ">" },
      {
        cN: "attribute",
        b: /\w+/,
        r: 0,
        k: {
          nomarkup:
            "order deny allow setenv rewriterule rewriteengine rewritecond documentroot sethandler errordocument loadmodule options header listen serverroot servername",
        },
        starts: {
          e: /$/,
          r: 0,
          k: { literal: "on off all" },
          c: [
            { cN: "meta", b: "\\s\\[", e: "\\]$" },
            { cN: "variable", b: "[\\$%]\\{", e: "\\}", c: ["self", r] },
            r,
            e.QSM,
          ],
        },
      },
    ],
    i: /\S/,
  };
});
hljs.registerLanguage("kotlin", function (e) {
  var t = {
      keyword:
        "abstract as val var vararg get set class object open private protected public noinline crossinline dynamic final enum if else do while for when throw try catch finally import package is in fun override companion reified inline lateinit init interface annotation data sealed internal infix operator out by constructor super tailrec where const inner suspend typealias external expect actual trait volatile transient native default",
      built_in:
        "Byte Short Char Int Long Boolean Float Double Void Unit Nothing",
      literal: "true false null",
    },
    r = { cN: "symbol", b: e.UIR + "@" },
    a = { cN: "subst", b: "\\${", e: "}", c: [e.ASM, e.CNM] },
    i = { cN: "variable", b: "\\$" + e.UIR },
    n = {
      cN: "string",
      v: [
        { b: '"""', e: '"""', c: [i, a] },
        { b: "'", e: "'", i: /\n/, c: [e.BE] },
        { b: '"', e: '"', i: /\n/, c: [e.BE, i, a] },
      ],
    },
    c = {
      cN: "meta",
      b:
        "@(?:file|property|field|get|set|receiver|param|setparam|delegate)\\s*:(?:\\s*" +
        e.UIR +
        ")?",
    },
    s = {
      cN: "meta",
      b: "@" + e.UIR,
      c: [{ b: /\(/, e: /\)/, c: [e.inherit(n, { cN: "meta-string" })] }],
    },
    l = {
      cN: "number",
      b: "\\b(0[bB]([01]+[01_]+[01]+|[01]+)|0[xX]([a-fA-F0-9]+[a-fA-F0-9_]+[a-fA-F0-9]+|[a-fA-F0-9]+)|(([\\d]+[\\d_]+[\\d]+|[\\d]+)(\\.([\\d]+[\\d_]+[\\d]+|[\\d]+))?|\\.([\\d]+[\\d_]+[\\d]+|[\\d]+))([eE][-+]?\\d+)?)[lLfF]?",
      r: 0,
    },
    b = e.C("/\\*", "\\*/", { c: [e.CBCM] }),
    o = {
      v: [
        { cN: "type", b: e.UIR },
        { b: /\(/, e: /\)/, c: [] },
      ],
    },
    d = o;
  return (
    (d.v[1].c = [o]),
    (o.v[1].c = [d]),
    {
      aliases: ["kt"],
      k: t,
      c: [
        e.C("/\\*\\*", "\\*/", {
          r: 0,
          c: [{ cN: "doctag", b: "@[A-Za-z]+" }],
        }),
        e.CLCM,
        b,
        {
          cN: "keyword",
          b: /\b(break|continue|return|this)\b/,
          starts: { c: [{ cN: "symbol", b: /@\w+/ }] },
        },
        r,
        c,
        s,
        {
          cN: "function",
          bK: "fun",
          e: "[(]|$",
          rB: !0,
          eE: !0,
          k: t,
          i: /fun\s+(<.*>)?[^\s\(]+(\s+[^\s\(]+)\s*=/,
          r: 5,
          c: [
            { b: e.UIR + "\\s*\\(", rB: !0, r: 0, c: [e.UTM] },
            { cN: "type", b: /</, e: />/, k: "reified", r: 0 },
            {
              cN: "params",
              b: /\(/,
              e: /\)/,
              endsParent: !0,
              k: t,
              r: 0,
              c: [
                { b: /:/, e: /[=,\/]/, eW: !0, c: [o, e.CLCM, b], r: 0 },
                e.CLCM,
                b,
                c,
                s,
                n,
                e.CNM,
              ],
            },
            b,
          ],
        },
        {
          cN: "class",
          bK: "class interface trait",
          e: /[:\{(]|$/,
          eE: !0,
          i: "extends implements",
          c: [
            { bK: "public protected internal private constructor" },
            e.UTM,
            { cN: "type", b: /</, e: />/, eB: !0, eE: !0, r: 0 },
            { cN: "type", b: /[,:]\s*/, e: /[<\(,]|$/, eB: !0, rE: !0 },
            c,
            s,
          ],
        },
        n,
        { cN: "meta", b: "^#!/usr/bin/env", e: "$", i: "\n" },
        l,
      ],
    }
  );
});
hljs.registerLanguage("xml", function (s) {
  var e = {
    eW: !0,
    i: /</,
    r: 0,
    c: [
      { cN: "attr", b: "[A-Za-z0-9\\._:-]+", r: 0 },
      {
        b: /=\s*/,
        r: 0,
        c: [
          {
            cN: "string",
            endsParent: !0,
            v: [{ b: /"/, e: /"/ }, { b: /'/, e: /'/ }, { b: /[^\s"'=<>`]+/ }],
          },
        ],
      },
    ],
  };
  return {
    aliases: [
      "html",
      "xhtml",
      "rss",
      "atom",
      "xjb",
      "xsd",
      "xsl",
      "plist",
      "wsf",
    ],
    cI: !0,
    c: [
      {
        cN: "meta",
        b: "<!DOCTYPE",
        e: ">",
        r: 10,
        c: [{ b: "\\[", e: "\\]" }],
      },
      s.C("\x3c!--", "--\x3e", { r: 10 }),
      { b: "<\\!\\[CDATA\\[", e: "\\]\\]>", r: 10 },
      { cN: "meta", b: /<\?xml/, e: /\?>/, r: 10 },
      {
        b: /<\?(php)?/,
        e: /\?>/,
        sL: "php",
        c: [
          { b: "/\\*", e: "\\*/", skip: !0 },
          { b: 'b"', e: '"', skip: !0 },
          { b: "b'", e: "'", skip: !0 },
          s.inherit(s.ASM, { i: null, cN: null, c: null, skip: !0 }),
          s.inherit(s.QSM, { i: null, cN: null, c: null, skip: !0 }),
        ],
      },
      {
        cN: "tag",
        b: "<style(?=\\s|>|$)",
        e: ">",
        k: { name: "style" },
        c: [e],
        starts: { e: "</style>", rE: !0, sL: ["css", "xml"] },
      },
      {
        cN: "tag",
        b: "<script(?=\\s|>|$)",
        e: ">",
        k: { name: "script" },
        c: [e],
        starts: {
          e: "</script>",
          rE: !0,
          sL: ["actionscript", "javascript", "handlebars", "xml", "vbscript"],
        },
      },
      {
        cN: "tag",
        b: "</?",
        e: "/?>",
        c: [{ cN: "name", b: /[^\/><\s]+/, r: 0 }, e],
      },
    ],
  };
});
hljs.registerLanguage("markdown", function (e) {
  return {
    aliases: ["md", "mkdown", "mkd"],
    c: [
      {
        cN: "section",
        v: [{ b: "^#{1,6}", e: "$" }, { b: "^.+?\\n[=-]{2,}$" }],
      },
      { b: "<", e: ">", sL: "xml", r: 0 },
      { cN: "bullet", b: "^\\s*([*+-]|(\\d+\\.))\\s+" },
      { cN: "strong", b: "[*_]{2}.+?[*_]{2}" },
      { cN: "emphasis", v: [{ b: "\\*.+?\\*" }, { b: "_.+?_", r: 0 }] },
      { cN: "quote", b: "^>\\s+", e: "$" },
      {
        cN: "code",
        v: [
          { b: "^```w*s*$", e: "^```s*$" },
          { b: "`.+?`" },
          { b: "^( {4}|\t)", e: "$", r: 0 },
        ],
      },
      { b: "^[-\\*]{3,}", e: "$" },
      {
        b: "\\[.+?\\][\\(\\[].*?[\\)\\]]",
        rB: !0,
        c: [
          { cN: "string", b: "\\[", e: "\\]", eB: !0, rE: !0, r: 0 },
          { cN: "link", b: "\\]\\(", e: "\\)", eB: !0, eE: !0 },
          { cN: "symbol", b: "\\]\\[", e: "\\]", eB: !0, eE: !0 },
        ],
        r: 10,
      },
      {
        b: /^\[[^\n]+\]:/,
        rB: !0,
        c: [
          { cN: "symbol", b: /\[/, e: /\]/, eB: !0, eE: !0 },
          { cN: "link", b: /:\s*/, e: /$/, eB: !0 },
        ],
      },
    ],
  };
});
hljs.registerLanguage("swift", function (e) {
  var i = {
      keyword:
        "#available #colorLiteral #column #else #elseif #endif #file #fileLiteral #function #if #imageLiteral #line #selector #sourceLocation _ __COLUMN__ __FILE__ __FUNCTION__ __LINE__ Any as as! as? associatedtype associativity break case catch class continue convenience default defer deinit didSet do dynamic dynamicType else enum extension fallthrough false fileprivate final for func get guard if import in indirect infix init inout internal is lazy left let mutating nil none nonmutating open operator optional override postfix precedence prefix private protocol Protocol public repeat required rethrows return right self Self set static struct subscript super switch throw throws true try try! try? Type typealias unowned var weak where while willSet",
      literal: "true false nil",
      built_in:
        "abs advance alignof alignofValue anyGenerator assert assertionFailure bridgeFromObjectiveC bridgeFromObjectiveCUnconditional bridgeToObjectiveC bridgeToObjectiveCUnconditional c contains count countElements countLeadingZeros debugPrint debugPrintln distance dropFirst dropLast dump encodeBitsAsWords enumerate equal fatalError filter find getBridgedObjectiveCType getVaList indices insertionSort isBridgedToObjectiveC isBridgedVerbatimToObjectiveC isUniquelyReferenced isUniquelyReferencedNonObjC join lazy lexicographicalCompare map max maxElement min minElement numericCast overlaps partition posix precondition preconditionFailure print println quickSort readLine reduce reflect reinterpretCast reverse roundUpToAlignment sizeof sizeofValue sort split startsWith stride strideof strideofValue swap toString transcode underestimateCount unsafeAddressOf unsafeBitCast unsafeDowncast unsafeUnwrap unsafeReflect withExtendedLifetime withObjectAtPlusZero withUnsafePointer withUnsafePointerToObject withUnsafeMutablePointer withUnsafeMutablePointers withUnsafePointer withUnsafePointers withVaList zip",
    },
    t = e.C("/\\*", "\\*/", { c: ["self"] }),
    n = { cN: "subst", b: /\\\(/, e: "\\)", k: i, c: [] },
    r = {
      cN: "string",
      c: [e.BE, n],
      v: [
        { b: /"""/, e: /"""/ },
        { b: /"/, e: /"/ },
      ],
    },
    a = {
      cN: "number",
      b: "\\b([\\d_]+(\\.[\\deE_]+)?|0x[a-fA-F0-9_]+(\\.[a-fA-F0-9p_]+)?|0b[01_]+|0o[0-7_]+)\\b",
      r: 0,
    };
  return (
    (n.c = [a]),
    {
      k: i,
      c: [
        r,
        e.CLCM,
        t,
        { cN: "type", b: "\\b[A-Z][\\wÀ-ʸ']*[!?]" },
        { cN: "type", b: "\\b[A-Z][\\wÀ-ʸ']*", r: 0 },
        a,
        {
          cN: "function",
          bK: "func",
          e: "{",
          eE: !0,
          c: [
            e.inherit(e.TM, { b: /[A-Za-z$_][0-9A-Za-z$_]*/ }),
            { b: /</, e: />/ },
            {
              cN: "params",
              b: /\(/,
              e: /\)/,
              endsParent: !0,
              k: i,
              c: ["self", a, r, e.CBCM, { b: ":" }],
              i: /["']/,
            },
          ],
          i: /\[|%/,
        },
        {
          cN: "class",
          bK: "struct protocol class extension enum",
          k: i,
          e: "\\{",
          eE: !0,
          c: [e.inherit(e.TM, { b: /[A-Za-z$_][\u00C0-\u02B80-9A-Za-z$_]*/ })],
        },
        {
          cN: "meta",
          b: "(@discardableResult|@warn_unused_result|@exported|@lazy|@noescape|@NSCopying|@NSManaged|@objc|@objcMembers|@convention|@required|@noreturn|@IBAction|@IBDesignable|@IBInspectable|@IBOutlet|@infix|@prefix|@postfix|@autoclosure|@testable|@available|@nonobjc|@NSApplicationMain|@UIApplicationMain)",
        },
        { bK: "import", e: /$/, c: [e.CLCM, t] },
      ],
    }
  );
});
hljs.registerLanguage("plaintext", function (e) {
  return { disableAutodetect: !0 };
});
hljs.registerLanguage("typescript", function (e) {
  var r = "[A-Za-z$_][0-9A-Za-z$_]*",
    t = {
      keyword:
        "in if for while finally var new function do return void else break catch instanceof with throw case default try this switch continue typeof delete let yield const class public private protected get set super static implements enum export import declare type namespace abstract as from extends async await",
      literal: "true false null undefined NaN Infinity",
      built_in:
        "eval isFinite isNaN parseFloat parseInt decodeURI decodeURIComponent encodeURI encodeURIComponent escape unescape Object Function Boolean Error EvalError InternalError RangeError ReferenceError StopIteration SyntaxError TypeError URIError Number Math Date String RegExp Array Float32Array Float64Array Int16Array Int32Array Int8Array Uint16Array Uint32Array Uint8Array Uint8ClampedArray ArrayBuffer DataView JSON Intl arguments require module console window document any number boolean string void Promise",
    },
    n = { cN: "meta", b: "@" + r },
    a = { b: "\\(", e: /\)/, k: t, c: ["self", e.QSM, e.ASM, e.NM] },
    s = {
      cN: "params",
      b: /\(/,
      e: /\)/,
      eB: !0,
      eE: !0,
      k: t,
      c: [e.CLCM, e.CBCM, n, a],
    },
    c = {
      cN: "number",
      v: [{ b: "\\b(0[bB][01]+)" }, { b: "\\b(0[oO][0-7]+)" }, { b: e.CNR }],
      r: 0,
    },
    o = { cN: "subst", b: "\\$\\{", e: "\\}", k: t, c: [] },
    i = {
      b: "html`",
      e: "",
      starts: { e: "`", rE: !1, c: [e.BE, o], sL: "xml" },
    },
    l = {
      b: "css`",
      e: "",
      starts: { e: "`", rE: !1, c: [e.BE, o], sL: "css" },
    },
    b = { cN: "string", b: "`", e: "`", c: [e.BE, o] };
  return (
    (o.c = [e.ASM, e.QSM, i, l, b, c, e.RM]),
    {
      aliases: ["ts"],
      k: t,
      c: [
        { cN: "meta", b: /^\s*['"]use strict['"]/ },
        e.ASM,
        e.QSM,
        i,
        l,
        b,
        e.CLCM,
        e.CBCM,
        c,
        {
          b: "(" + e.RSR + "|\\b(case|return|throw)\\b)\\s*",
          k: "return throw case",
          c: [
            e.CLCM,
            e.CBCM,
            e.RM,
            {
              cN: "function",
              b: "(\\(.*?\\)|" + e.IR + ")\\s*=>",
              rB: !0,
              e: "\\s*=>",
              c: [
                {
                  cN: "params",
                  v: [
                    { b: e.IR },
                    { b: /\(\s*\)/ },
                    {
                      b: /\(/,
                      e: /\)/,
                      eB: !0,
                      eE: !0,
                      k: t,
                      c: ["self", e.CLCM, e.CBCM],
                    },
                  ],
                },
              ],
            },
          ],
          r: 0,
        },
        {
          cN: "function",
          b: "function",
          e: /[\{;]/,
          eE: !0,
          k: t,
          c: ["self", e.inherit(e.TM, { b: r }), s],
          i: /%/,
          r: 0,
        },
        { bK: "constructor", e: /\{/, eE: !0, c: ["self", s] },
        { b: /module\./, k: { built_in: "module" }, r: 0 },
        { bK: "module", e: /\{/, eE: !0 },
        { bK: "interface", e: /\{/, eE: !0, k: "interface extends" },
        { b: /\$[(.]/ },
        { b: "\\." + e.IR, r: 0 },
        n,
        a,
      ],
    }
  );
});
hljs.registerLanguage("nginx", function (e) {
  var r = {
      cN: "variable",
      v: [{ b: /\$\d+/ }, { b: /\$\{/, e: /}/ }, { b: "[\\$\\@]" + e.UIR }],
    },
    b = {
      eW: !0,
      l: "[a-z/_]+",
      k: {
        literal:
          "on off yes no true false none blocked debug info notice warn error crit select break last permanent redirect kqueue rtsig epoll poll /dev/poll",
      },
      r: 0,
      i: "=>",
      c: [
        e.HCM,
        {
          cN: "string",
          c: [e.BE, r],
          v: [
            { b: /"/, e: /"/ },
            { b: /'/, e: /'/ },
          ],
        },
        { b: "([a-z]+):/", e: "\\s", eW: !0, eE: !0, c: [r] },
        {
          cN: "regexp",
          c: [e.BE, r],
          v: [
            { b: "\\s\\^", e: "\\s|{|;", rE: !0 },
            { b: "~\\*?\\s+", e: "\\s|{|;", rE: !0 },
            { b: "\\*(\\.[a-z\\-]+)+" },
            { b: "([a-z\\-]+\\.)+\\*" },
          ],
        },
        {
          cN: "number",
          b: "\\b\\d{1,3}\\.\\d{1,3}\\.\\d{1,3}\\.\\d{1,3}(:\\d{1,5})?\\b",
        },
        { cN: "number", b: "\\b\\d+[kKmMgGdshdwy]*\\b", r: 0 },
        r,
      ],
    };
  return {
    aliases: ["nginxconf"],
    c: [
      e.HCM,
      {
        b: e.UIR + "\\s+{",
        rB: !0,
        e: "{",
        c: [{ cN: "section", b: e.UIR }],
        r: 0,
      },
      {
        b: e.UIR + "\\s",
        e: ";|{",
        rB: !0,
        c: [{ cN: "attribute", b: e.UIR, starts: b }],
        r: 0,
      },
    ],
    i: "[^\\s\\}]",
  };
});
hljs.registerLanguage("go", function (e) {
  var t = {
    keyword:
      "break default func interface select case map struct chan else goto package switch const fallthrough if range type continue for import return var go defer bool byte complex64 complex128 float32 float64 int8 int16 int32 int64 string uint8 uint16 uint32 uint64 int uint uintptr rune",
    literal: "true false iota nil",
    built_in:
      "append cap close complex copy imag len make new panic print println real recover delete",
  };
  return {
    aliases: ["golang"],
    k: t,
    i: "</",
    c: [
      e.CLCM,
      e.CBCM,
      {
        cN: "string",
        v: [e.QSM, { b: "'", e: "[^\\\\]'" }, { b: "`", e: "`" }],
      },
      { cN: "number", v: [{ b: e.CNR + "[i]", r: 1 }, e.CNM] },
      { b: /:=/ },
      {
        cN: "function",
        bK: "func",
        e: /\s*\{/,
        eE: !0,
        c: [e.TM, { cN: "params", b: /\(/, e: /\)/, k: t, i: /["']/ }],
      },
    ],
  };
});
hljs.registerLanguage("javascript", function (e) {
  var r = "[A-Za-z$_][0-9A-Za-z$_]*",
    t = {
      keyword:
        "in of if for while finally var new function do return void else break catch instanceof with throw case default try this switch continue typeof delete let yield const export super debugger as async await static import from as",
      literal: "true false null undefined NaN Infinity",
      built_in:
        "eval isFinite isNaN parseFloat parseInt decodeURI decodeURIComponent encodeURI encodeURIComponent escape unescape Object Function Boolean Error EvalError InternalError RangeError ReferenceError StopIteration SyntaxError TypeError URIError Number Math Date String RegExp Array Float32Array Float64Array Int16Array Int32Array Int8Array Uint16Array Uint32Array Uint8Array Uint8ClampedArray ArrayBuffer DataView JSON Intl arguments require module console window document Symbol Set Map WeakSet WeakMap Proxy Reflect Promise",
    },
    a = {
      cN: "number",
      v: [{ b: "\\b(0[bB][01]+)" }, { b: "\\b(0[oO][0-7]+)" }, { b: e.CNR }],
      r: 0,
    },
    s = { cN: "subst", b: "\\$\\{", e: "\\}", k: t, c: [] },
    c = {
      b: "html`",
      e: "",
      starts: { e: "`", rE: !1, c: [e.BE, s], sL: "xml" },
    },
    n = {
      b: "css`",
      e: "",
      starts: { e: "`", rE: !1, c: [e.BE, s], sL: "css" },
    },
    o = { cN: "string", b: "`", e: "`", c: [e.BE, s] };
  s.c = [e.ASM, e.QSM, c, n, o, a, e.RM];
  var i = s.c.concat([e.CBCM, e.CLCM]);
  return {
    aliases: ["js", "jsx"],
    k: t,
    c: [
      { cN: "meta", r: 10, b: /^\s*['"]use (strict|asm)['"]/ },
      { cN: "meta", b: /^#!/, e: /$/ },
      e.ASM,
      e.QSM,
      c,
      n,
      o,
      e.CLCM,
      e.CBCM,
      a,
      {
        b: /[{,]\s*/,
        r: 0,
        c: [{ b: r + "\\s*:", rB: !0, r: 0, c: [{ cN: "attr", b: r, r: 0 }] }],
      },
      {
        b: "(" + e.RSR + "|\\b(case|return|throw)\\b)\\s*",
        k: "return throw case",
        c: [
          e.CLCM,
          e.CBCM,
          e.RM,
          {
            cN: "function",
            b: "(\\(.*?\\)|" + r + ")\\s*=>",
            rB: !0,
            e: "\\s*=>",
            c: [
              {
                cN: "params",
                v: [
                  { b: r },
                  { b: /\(\s*\)/ },
                  { b: /\(/, e: /\)/, eB: !0, eE: !0, k: t, c: i },
                ],
              },
            ],
          },
          { cN: "", b: /\s/, e: /\s*/, skip: !0 },
          {
            b: /</,
            e: /(\/[A-Za-z0-9\\._:-]+|[A-Za-z0-9\\._:-]+\/)>/,
            sL: "xml",
            c: [
              { b: /<[A-Za-z0-9\\._:-]+\s*\/>/, skip: !0 },
              {
                b: /<[A-Za-z0-9\\._:-]+/,
                e: /(\/[A-Za-z0-9\\._:-]+|[A-Za-z0-9\\._:-]+\/)>/,
                skip: !0,
                c: [{ b: /<[A-Za-z0-9\\._:-]+\s*\/>/, skip: !0 }, "self"],
              },
            ],
          },
        ],
        r: 0,
      },
      {
        cN: "function",
        bK: "function",
        e: /\{/,
        eE: !0,
        c: [
          e.inherit(e.TM, { b: r }),
          { cN: "params", b: /\(/, e: /\)/, eB: !0, eE: !0, c: i },
        ],
        i: /\[|%/,
      },
      { b: /\$[(.]/ },
      e.METHOD_GUARD,
      {
        cN: "class",
        bK: "class",
        e: /[{;=]/,
        eE: !0,
        i: /[:"\[\]]/,
        c: [{ bK: "extends" }, e.UTM],
      },
      { bK: "constructor get set", e: /\{/, eE: !0 },
    ],
    i: /#(?!!)/,
  };
});
hljs.registerLanguage("php", function (e) {
  var c = { b: "\\$+[a-zA-Z_-ÿ][a-zA-Z0-9_-ÿ]*" },
    i = { cN: "meta", b: /<\?(php)?|\?>/ },
    t = {
      cN: "string",
      c: [e.BE, i],
      v: [
        { b: 'b"', e: '"' },
        { b: "b'", e: "'" },
        e.inherit(e.ASM, { i: null }),
        e.inherit(e.QSM, { i: null }),
      ],
    },
    a = { v: [e.BNM, e.CNM] };
  return {
    aliases: ["php", "php3", "php4", "php5", "php6", "php7"],
    cI: !0,
    k: "and include_once list abstract global private echo interface as static endswitch array null if endwhile or const for endforeach self var while isset public protected exit foreach throw elseif include __FILE__ empty require_once do xor return parent clone use __CLASS__ __LINE__ else break print eval new catch __METHOD__ case exception default die require __FUNCTION__ enddeclare final try switch continue endfor endif declare unset true false trait goto instanceof insteadof __DIR__ __NAMESPACE__ yield finally",
    c: [
      e.HCM,
      e.C("//", "$", { c: [i] }),
      e.C("/\\*", "\\*/", { c: [{ cN: "doctag", b: "@[A-Za-z]+" }] }),
      e.C("__halt_compiler.+?;", !1, {
        eW: !0,
        k: "__halt_compiler",
        l: e.UIR,
      }),
      {
        cN: "string",
        b: /<<<['"]?\w+['"]?$/,
        e: /^\w+;?$/,
        c: [e.BE, { cN: "subst", v: [{ b: /\$\w+/ }, { b: /\{\$/, e: /\}/ }] }],
      },
      i,
      { cN: "keyword", b: /\$this\b/ },
      c,
      { b: /(::|->)+[a-zA-Z_\x7f-\xff][a-zA-Z0-9_\x7f-\xff]*/ },
      {
        cN: "function",
        bK: "function",
        e: /[;{]/,
        eE: !0,
        i: "\\$|\\[|%",
        c: [
          e.UTM,
          { cN: "params", b: "\\(", e: "\\)", c: ["self", c, e.CBCM, t, a] },
        ],
      },
      {
        cN: "class",
        bK: "class interface",
        e: "{",
        eE: !0,
        i: /[:\(\$"]/,
        c: [{ bK: "extends implements" }, e.UTM],
      },
      { bK: "namespace", e: ";", i: /[\.']/, c: [e.UTM] },
      { bK: "use", e: ";", c: [e.UTM] },
      { b: "=>" },
      t,
      a,
    ],
  };
});
hljs.registerLanguage("cs", function (e) {
  var i = {
      keyword:
        "abstract as base bool break byte case catch char checked const continue decimal default delegate do double enum event explicit extern finally fixed float for foreach goto if implicit in int interface internal is lock long nameof object operator out override params private protected public readonly ref sbyte sealed short sizeof stackalloc static string struct switch this try typeof uint ulong unchecked unsafe ushort using virtual void volatile while add alias ascending async await by descending dynamic equals from get global group into join let on orderby partial remove select set value var where yield",
      literal: "null false true",
    },
    r = {
      cN: "number",
      v: [
        { b: "\\b(0b[01']+)" },
        {
          b: "(-?)\\b([\\d']+(\\.[\\d']*)?|\\.[\\d']+)(u|U|l|L|ul|UL|f|F|b|B)",
        },
        {
          b: "(-?)(\\b0[xX][a-fA-F0-9']+|(\\b[\\d']+(\\.[\\d']*)?|\\.[\\d']+)([eE][-+]?[\\d']+)?)",
        },
      ],
      r: 0,
    },
    t = { cN: "string", b: '@"', e: '"', c: [{ b: '""' }] },
    a = e.inherit(t, { i: /\n/ }),
    c = { cN: "subst", b: "{", e: "}", k: i },
    n = e.inherit(c, { i: /\n/ }),
    s = {
      cN: "string",
      b: /\$"/,
      e: '"',
      i: /\n/,
      c: [{ b: "{{" }, { b: "}}" }, e.BE, n],
    },
    b = {
      cN: "string",
      b: /\$@"/,
      e: '"',
      c: [{ b: "{{" }, { b: "}}" }, { b: '""' }, c],
    },
    l = e.inherit(b, {
      i: /\n/,
      c: [{ b: "{{" }, { b: "}}" }, { b: '""' }, n],
    });
  (c.c = [b, s, t, e.ASM, e.QSM, r, e.CBCM]),
    (n.c = [l, s, a, e.ASM, e.QSM, r, e.inherit(e.CBCM, { i: /\n/ })]);
  var o = { v: [b, s, t, e.ASM, e.QSM] },
    d = e.IR + "(<" + e.IR + "(\\s*,\\s*" + e.IR + ")*>)?(\\[\\])?";
  return {
    aliases: ["csharp", "c#"],
    k: i,
    i: /::/,
    c: [
      e.C("///", "$", {
        rB: !0,
        c: [
          {
            cN: "doctag",
            v: [
              { b: "///", r: 0 },
              { b: "\x3c!--|--\x3e" },
              { b: "</?", e: ">" },
            ],
          },
        ],
      }),
      e.CLCM,
      e.CBCM,
      {
        cN: "meta",
        b: "#",
        e: "$",
        k: {
          "meta-keyword":
            "if else elif endif define undef warning error line region endregion pragma checksum",
        },
      },
      o,
      r,
      {
        bK: "class interface",
        e: /[{;=]/,
        i: /[^\s:,]/,
        c: [e.TM, e.CLCM, e.CBCM],
      },
      {
        bK: "namespace",
        e: /[{;=]/,
        i: /[^\s:]/,
        c: [e.inherit(e.TM, { b: "[a-zA-Z](\\.?\\w)*" }), e.CLCM, e.CBCM],
      },
      {
        cN: "meta",
        b: "^\\s*\\[",
        eB: !0,
        e: "\\]",
        eE: !0,
        c: [{ cN: "meta-string", b: /"/, e: /"/ }],
      },
      { bK: "new return throw await else", r: 0 },
      {
        cN: "function",
        b: "(" + d + "\\s+)+" + e.IR + "\\s*\\(",
        rB: !0,
        e: /\s*[{;=]/,
        eE: !0,
        k: i,
        c: [
          { b: e.IR + "\\s*\\(", rB: !0, c: [e.TM], r: 0 },
          {
            cN: "params",
            b: /\(/,
            e: /\)/,
            eB: !0,
            eE: !0,
            k: i,
            r: 0,
            c: [o, r, e.CBCM],
          },
          e.CLCM,
          e.CBCM,
        ],
      },
    ],
  };
});
hljs.registerLanguage("lua", function (e) {
  var t = "\\[=*\\[",
    a = "\\]=*\\]",
    r = { b: t, e: a, c: ["self"] },
    n = [e.C("--(?!" + t + ")", "$"), e.C("--" + t, a, { c: [r], r: 10 })];
  return {
    l: e.UIR,
    k: {
      literal: "true false nil",
      keyword:
        "and break do else elseif end for goto if in local not or repeat return then until while",
      built_in:
        "_G _ENV _VERSION __index __newindex __mode __call __metatable __tostring __len __gc __add __sub __mul __div __mod __pow __concat __unm __eq __lt __le assert collectgarbage dofile error getfenv getmetatable ipairs load loadfile loadstringmodule next pairs pcall print rawequal rawget rawset require select setfenvsetmetatable tonumber tostring type unpack xpcall arg selfcoroutine resume yield status wrap create running debug getupvalue debug sethook getmetatable gethook setmetatable setlocal traceback setfenv getinfo setupvalue getlocal getregistry getfenv io lines write close flush open output type read stderr stdin input stdout popen tmpfile math log max acos huge ldexp pi cos tanh pow deg tan cosh sinh random randomseed frexp ceil floor rad abs sqrt modf asin min mod fmod log10 atan2 exp sin atan os exit setlocale date getenv difftime remove time clock tmpname rename execute package preload loadlib loaded loaders cpath config path seeall string sub upper len gfind rep find match char dump gmatch reverse byte format gsub lower table setn insert getn foreachi maxn foreach concat sort remove",
    },
    c: n.concat([
      {
        cN: "function",
        bK: "function",
        e: "\\)",
        c: [
          e.inherit(e.TM, {
            b: "([_a-zA-Z]\\w*\\.)*([_a-zA-Z]\\w*:)?[_a-zA-Z]\\w*",
          }),
          { cN: "params", b: "\\(", eW: !0, c: n },
        ].concat(n),
      },
      e.CNM,
      e.ASM,
      e.QSM,
      { cN: "string", b: t, e: a, c: [r], r: 5 },
    ]),
  };
});
hljs.registerLanguage("cpp", function (t) {
  var e = { cN: "keyword", b: "\\b[a-z\\d_]*_t\\b" },
    r = {
      cN: "string",
      v: [
        { b: '(u8?|U|L)?"', e: '"', i: "\\n", c: [t.BE] },
        { b: /(?:u8?|U|L)?R"([^()\\ ]{0,16})\((?:.|\n)*?\)\1"/ },
        { b: "'\\\\?.", e: "'", i: "." },
      ],
    },
    s = {
      cN: "number",
      v: [
        { b: "\\b(0b[01']+)" },
        {
          b: "(-?)\\b([\\d']+(\\.[\\d']*)?|\\.[\\d']+)(u|U|l|L|ul|UL|f|F|b|B)",
        },
        {
          b: "(-?)(\\b0[xX][a-fA-F0-9']+|(\\b[\\d']+(\\.[\\d']*)?|\\.[\\d']+)([eE][-+]?[\\d']+)?)",
        },
      ],
      r: 0,
    },
    i = {
      cN: "meta",
      b: /#\s*[a-z]+\b/,
      e: /$/,
      k: {
        "meta-keyword":
          "if else elif endif define undef warning error line pragma ifdef ifndef include",
      },
      c: [
        { b: /\\\n/, r: 0 },
        t.inherit(r, { cN: "meta-string" }),
        { cN: "meta-string", b: /<[^\n>]*>/, e: /$/, i: "\\n" },
        t.CLCM,
        t.CBCM,
      ],
    },
    a = t.IR + "\\s*\\(",
    c = {
      keyword:
        "int float while private char catch import module export virtual operator sizeof dynamic_cast|10 typedef const_cast|10 const for static_cast|10 union namespace unsigned long volatile static protected bool template mutable if public friend do goto auto void enum else break extern using asm case typeid short reinterpret_cast|10 default double register explicit signed typename try this switch continue inline delete alignof constexpr decltype noexcept static_assert thread_local restrict _Bool complex _Complex _Imaginary atomic_bool atomic_char atomic_schar atomic_uchar atomic_short atomic_ushort atomic_int atomic_uint atomic_long atomic_ulong atomic_llong atomic_ullong new throw return and or not",
      built_in:
        "std string cin cout cerr clog stdin stdout stderr stringstream istringstream ostringstream auto_ptr deque list queue stack vector map set bitset multiset multimap unordered_set unordered_map unordered_multiset unordered_multimap array shared_ptr abort abs acos asin atan2 atan calloc ceil cosh cos exit exp fabs floor fmod fprintf fputs free frexp fscanf isalnum isalpha iscntrl isdigit isgraph islower isprint ispunct isspace isupper isxdigit tolower toupper labs ldexp log10 log malloc realloc memchr memcmp memcpy memset modf pow printf putchar puts scanf sinh sin snprintf sprintf sqrt sscanf strcat strchr strcmp strcpy strcspn strlen strncat strncmp strncpy strpbrk strrchr strspn strstr tanh tan vfprintf vprintf vsprintf endl initializer_list unique_ptr",
      literal: "true false nullptr NULL",
    },
    n = [e, t.CLCM, t.CBCM, s, r];
  return {
    aliases: ["c", "cc", "h", "c++", "h++", "hpp", "hh", "hxx", "cxx"],
    k: c,
    i: "</",
    c: n.concat([
      i,
      {
        b: "\\b(deque|list|queue|stack|vector|map|set|bitset|multiset|multimap|unordered_map|unordered_set|unordered_multiset|unordered_multimap|array)\\s*<",
        e: ">",
        k: c,
        c: ["self", e],
      },
      { b: t.IR + "::", k: c },
      {
        v: [
          { b: /=/, e: /;/ },
          { b: /\(/, e: /\)/ },
          { bK: "new throw return else", e: /;/ },
        ],
        k: c,
        c: n.concat([{ b: /\(/, e: /\)/, k: c, c: n.concat(["self"]), r: 0 }]),
        r: 0,
      },
      {
        cN: "function",
        b: "(" + t.IR + "[\\*&\\s]+)+" + a,
        rB: !0,
        e: /[{;=]/,
        eE: !0,
        k: c,
        i: /[^\w\s\*&]/,
        c: [
          { b: a, rB: !0, c: [t.TM], r: 0 },
          {
            cN: "params",
            b: /\(/,
            e: /\)/,
            k: c,
            r: 0,
            c: [
              t.CLCM,
              t.CBCM,
              r,
              s,
              e,
              {
                b: /\(/,
                e: /\)/,
                k: c,
                r: 0,
                c: ["self", t.CLCM, t.CBCM, r, s, e],
              },
            ],
          },
          t.CLCM,
          t.CBCM,
          i,
        ],
      },
      {
        cN: "class",
        bK: "class struct",
        e: /[{;:]/,
        c: [{ b: /</, e: />/, c: ["self"] }, t.TM],
      },
    ]),
    exports: { preprocessor: i, strings: r, k: c },
  };
});
