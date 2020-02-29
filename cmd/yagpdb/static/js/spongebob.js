lastLoc = window.location.pathname;
lastHash = window.location.hash;
$(function () {

	$("#loading-overlay").addClass("hidden");


	if (visibleURL) {
		console.log("Should navigate to", visibleURL);
		window.history.replaceState("", "", visibleURL);
	}

	addListeners();
	initPlugins(false);

	window.onpopstate = function (evt, a) {
		var shouldNav;
		console.log(window.location.pathname);
		console.log(lastLoc);
		if (window.location.pathname !== lastLoc) {
			shouldNav = true;
		} else {
			shouldNav = false;
		}

		console.log("Popped state", shouldNav, evt, evt.path);
		if (shouldNav) {
			navigate(window.location.pathname, "GET", null, false)
		}
		// Handle the back (or forward) buttons here
		// Will NOT handle refresh, use onbeforeunload for this.
	};

	if (window.location.hash) {
		navigateToAnchor(window.location.hash);
	}

	updateSelectedMenuItem(window.location.pathname);


	// Update all dropdowns
	// $(".btn-group .dropdown-menu").dropdownUpdate();
})

var currentlyLoading = false;
function navigate(url, method, data, updateHistory, maintainScroll, alertsOnly, cb) {
	if (currentlyLoading) { return; }
	closeSidebar();

	var scrollBeforeNav = document.documentElement.scrollTop;

	$("#loading-overlay").removeClass("hidden");
	// $("#main-content").html('<div class="loader">Loading...</div>');

	currentlyLoading = true;
	var evt = new CustomEvent('customnavigate', { url: url });
	window.dispatchEvent(evt);

	if (url[0] !== "/") {
		url = window.location.pathname + url;
	}

	console.log("Navigating to " + url);
	var shownURL = url;
	// Add the partial param
	var index = url.indexOf("?")
	if (index !== -1) {
		url += "&partial=1"
	} else {
		url += "?partial=1"
	}

	if (alertsOnly) {
		url += "&alertsonly=1"
	}

	PNotify.removeAll();

	updateSelectedMenuItem(url);

	var req = new XMLHttpRequest();
	req.addEventListener("load", function () {
		currentlyLoading = false;
		if (this.status !== 200 && this.status !== 400) {
			window.location.href = '/';
			return;
		} else if (this.status === 400) {
			alertsOnly = true;
		}

		if (updateHistory) {
			window.history.pushState("", "", shownURL);
		}
		lastLoc = shownURL;
		lastHash = window.location.hash;

		if (alertsOnly) {
			showAlerts(this.responseText)
			$("#loading-overlay").addClass("hidden");
			if (cb)
				cb();
			return
		}

		$("#main-content").html(this.responseText);

		initPlugins(true);
		$(document.body).trigger('ready');

		if (typeof ga !== 'undefined') {
			ga('send', 'pageview', window.location.pathname);
			console.log("Sent pageview")
		}

		if (cb)
			cb();

		if (maintainScroll)
			document.documentElement.scrollTop = scrollBeforeNav;

		$("#loading-overlay").addClass("hidden");
	});

	req.addEventListener("error", function () {
		window.location.href = '/';
		currentlyLoading = false;
	});

	req.open(method, url);
	req.setRequestHeader('Cache-Control', 'no-cache');

	if (data) {
		req.setRequestHeader("content-type", "application/x-www-form-urlencoded");
		req.send(data);
	} else {
		req.send();
	}
}

function showAlerts(alertsJson) {
	var alerts = JSON.parse(alertsJson);
	if (!alerts) return;

	const stack_bar_top = { "dir1": "down", "dir2": "right", "push": "top", "spacing1": 0, "spacing2": 0 };

	for (var i = 0; i < alerts.length; i++) {
		var alert = alerts[i];

		var notice;
		if (alert.Style === "success") {
			notice = new PNotify({
				title: alert.Message,
				type: 'success',
				addclass: 'stack-bar-top click-2-close',
				stack: stack_bar_top,
				width: "100%",
				delay: 2000,
				buttons: {
					closer: false,
					sticker: false
				}
			});
		} else if (alert.Style === "danger") {
			notice = new PNotify({
				title: alert.Message,
				text: "Read the docs and contact support if you don't know what went wrong.",
				type: 'error',
				addclass: 'stack-bar-top click-2-close',
				stack: stack_bar_top,
				width: "100%",
				hide: false,
				buttons: {
					closer: false,
					sticker: false
				}
			});
		} else {
			continue;
		}

		(function () {
			var noticeCop = notice;
			noticeCop.get().click(function () {
				noticeCop.remove();
			});
		})()
	}
}

function closeSidebar() {
	document.documentElement.classList.remove("sidebar-left-opened");

	$(window).trigger("sidebar-left-opened", {
		added: false,
		removed: true
	});
}

// Automatically marks the the menu entry corresponding with our active page as active
function updateSelectedMenuItem(pathname) {
	// Collapse all nav parents first
	var navParents = document.querySelectorAll("#menu .nav-parent");
	for (var i = 0; i < navParents.length; i++) {
		navParents[i].classList.remove("nav-expanded", "nav-active");
	}

	// Then update the nav links
	var navLinks = document.querySelectorAll("#menu .nav-link")

	var bestMatch = -1;
	var bestMatchLength = 0;
	for (var i = 0; i < navLinks.length; i++) {
		var href = navLinks[i].attributes.getNamedItem("href").value;
		if (pathname.indexOf(href) !== -1) {
			if (href.length > bestMatchLength) {
				bestMatch = i;
				bestMatchLength = href.length
			}
		}

		navLinks[i].parentElement.classList.remove("nav-active");
	}

	if (bestMatch !== -1) {
		var collapseParent = navLinks[bestMatch].parentElement.parentElement.parentElement;
		if (collapseParent.classList.contains("nav-parent")) {
			collapseParent.classList.add("nav-expanded", "nav-active");
		}

		navLinks[bestMatch].parentElement.classList.add("nav-active");
	}
}

function addAlert(kind, msg) {
	$("<div/>").addClass("row").append(
		$("<div/>").addClass("col-lg-12").append(
			$("<div/>").addClass("alert alert-" + kind).text(msg)
		)
	).appendTo("#alerts");
}

function clearAlerts() {
	$("#alerts").empty();
}

function addListeners() {
	////////////////////////////////////////
	// Async partial page loading handling
	///////////////////////////////////////

	formSubmissionEvents();

	$(document).on("click", '[data-partial-load]', function (event) {
		console.log("Clicked the link");
		event.preventDefault();

		if (currentlyLoading) { return; }

		var link = $(this);

		var url = link.attr("href");
		navigate(url, "GET", null, true);
	});

	$(document).on("click", '[data-toggle="popover"]', function (evt) {
		$('[data-toggle="popover"]').each(function (i, elem) {
			// console.log(elem, elem == evt.target);
			if (evt.currentTarget == elem) {
				return;
			}
			$(elem).popover('hide');
		})
	});

	$(document).on("click", 'a[href^="#"]', function (e) {
		//e.preventDefault();

		navigateToAnchor($.attr(this, "href"));
	})


	$(document).on('click', '.btn-add', function (e) {
		e.preventDefault();

		var currentEntry = $(this).parent().parent(),
			newEntry = $(currentEntry.clone()).insertAfter(currentEntry);

		newEntry.find('input, textarea').val('');
		newEntry.parent().find('.entry:not(:last-of-type) .btn-add')
			.removeClass('btn-add').addClass('btn-remove')
			.removeClass('btn-success').addClass('btn-danger')
			.html('<i class="fas fa-minus"></i>');
	}).on('click', '.btn-remove', function (e) {
		$(this).parents('.entry:first').remove();

		e.preventDefault();
		return false;
	});

	$(document).on('click', '.modal-dismiss', function (e) {
		e.preventDefault();
		$.magnificPopup.close();
	});

	$(window).on("sidebar-left-toggle", function (evt, data) {
		window.localStorage.setItem("sidebar_collapsed", !data.removed);
		document.cookie = "sidebar_collapsed=" + data.added + "; max-age=3153600000; path=/"
	})
}

// Initializes plugins such as multiselect, we have to do this on the new elements each time we load a partial page
function initPlugins(partial) {
	var selectorPrefix = "";
	if (partial) {
		selectorPrefix = "#main-content ";
	}

	$(selectorPrefix + '[data-toggle="popover"]').popover()
	$(selectorPrefix + '[data-toggle="tooltip"]').tooltip();

	$('.entry:not(:last-of-type) .btn-add')
		.removeClass('btn-add').addClass('btn-remove')
		.removeClass('btn-success').addClass('btn-danger')
		.html('<i class="fas fa-minus"></i>');

	// The uitlity that checks wether the bot has permissions to send messages in the selected channel
	channelRequirepermsDropdown(selectorPrefix);
	yagInitSelect2(selectorPrefix)
	yagInitMultiSelect(selectorPrefix)
	yagInitAutosize(selectorPrefix);
	yagInitUnsavedForms(selectorPrefix)
	// initializeMultiselect(selectorPrefix);

	$(selectorPrefix + '.modal-basic').magnificPopup({
		type: 'inline',
		preloader: false,
		modal: true
	});
}

var discordPermissions = {
	read: {
		name: "Read Messages",
		perm: 0x400
	},
	send: {
		name: "Send Messages",
		perm: 0x800
	},
	embed: {
		name: "Embed Links",
		perm: 0x4000
	},
}
var cachedChannelPerms = {};
function channelRequirepermsDropdown(prefix) {
	var dropdowns = $(prefix + "select[data-requireperms-send]");
	dropdowns.each(function (i, rawElem) {
		trackChannelDropdown($(rawElem), [discordPermissions.read, discordPermissions.send]);
	});

	var dropdownsLinks = $(prefix + "select[data-requireperms-embed]");
	dropdownsLinks.each(function (i, rawElem) {
		trackChannelDropdown($(rawElem), [discordPermissions.read, discordPermissions.send, discordPermissions.embed]);
	});
}

function trackChannelDropdown(dropdown, perms) {
	var currentElem = $('<p class="form-control-static">Checking channel permissions for bot...</p>');
	dropdown.after(currentElem);

	dropdown.on("change", function () {
		check();
	})

	function check() {
		currentElem.text("Checking channel permissions for bot...");
		currentElem.removeClass("text-success", "text-danger");
		var currentSelected = dropdown.val();
		if (!currentSelected) {
			currentElem.text("");
		} else {
			validateChannelDropdown(dropdown, currentElem, currentSelected, perms);
		}
	}
	check();
}

function validateChannelDropdown(dropdown, currentElem, channel, perms) {
	// Expire after 5 seconds
	if (cachedChannelPerms[channel] && (!cachedChannelPerms[channel].lastChecked || Date.now() - cachedChannelPerms[channel].lastChecked < 5000)) {
		var obj = cachedChannelPerms[channel];
		if (obj.fetching) {
			window.setTimeout(function () {
				validateChannelDropdown(dropdown, currentElem, channel, perms);
			}, 1000)
		} else {
			check(cachedChannelPerms[channel].perms);
		}
	} else {
		cachedChannelPerms[channel] = { fetching: true };
		createRequest("GET", "/api/" + CURRENT_GUILDID + "/channelperms/" + channel, null, function () {
			console.log(this);
			cachedChannelPerms[channel].fetching = false;
			if (this.status != 200) {
				currentElem.addClass("text-danger");
				currentElem.removeClass("text-success");

				if (this.responseText) {
					var decoded = JSON.parse(this.responseText);
					if (decoded.message) {
						currentElem.text(decoded.message);
					} else {
						currentElem.text("Couldn't check permissions :(");
					}
				} else {
					currentElem.text("Couldn't check permissions :(");
				}
				cachedChannelPerms[channel] = null;
				return;
			}

			var channelPerms = parseInt(this.responseText);
			cachedChannelPerms[channel].perms = channelPerms;
			cachedChannelPerms[channel].lastChecked = Date.now();

			check(channelPerms);
		})
	}

	function check(channelPerms) {
		var missing = [];
		for (var i in perms) {
			var p = perms[i];
			if ((channelPerms & p.perm) != p.perm) {
				missing.push(p.name);
			}
		}

		// console.log(missing.join(", "));
		if (missing.length < 1) {
			// Has perms
			currentElem.removeClass("text-danger");
			currentElem.addClass("text-success");
			currentElem.text("");
		} else {
			currentElem.addClass("text-danger");
			currentElem.removeClass("text-success");

			currentElem.text("Missing " + missing.join(", "));
		}
	}
}

function initializeMultiselect(selectorPrefix) {
	// $(selectorPrefix+".multiselect").multiselect();
}

function formSubmissionEvents() {
	// forms.each(function(i, elem){
	// 	elem.onsubmit = submitform;
	// })

	function dangerButtonClick(evt) {
		var target = $(evt.target);
		if (target.prop("tagName") !== "BUTTON") {
			target = target.parents("button");
			if (!target) {
				return
			}
		}

		if (target.attr("noconfirm") !== undefined) {
			return;
		}

		if (target.attr("noconfirm")) {
			return
		}

		if (target.attr("formaction")) {
			return;
		}

		// console.log("aaaaa", evt, evt.preventDefault);
		if (!confirm("Are you sure you want to do this?")) {
			evt.preventDefault(true);
			evt.stopPropagation();
		}
		// alert("aaa")
	}

	$(document).on("click", ".btn-danger", dangerButtonClick);
	$(document).on("click", ".delete-button", dangerButtonClick);

	function getRandomInt(min, max) {
		min = Math.ceil(min);
		max = Math.floor(max);
		return Math.floor(Math.random() * (max - min)) + min;
	}


	$(document).on("submit", '[data-async-form]', function (event) {
		// console.log("Clicked the link");
		event.preventDefault();

		var action = $(event.target).attr("action");
		if (!action) {
			action = window.location.pathname;
		}

		submitForm($(event.target), action, false);
	});

	$(document).on("click", 'button', function (event) {
		// console.log("Clicked the link");
		var target = $(event.target);

		if (target.prop("tagName") !== "BUTTON") {
			target = target.parents("button");
		}

		var alertsOnly = false;
		if (target.attr("data-async-form-alertsonly") !== undefined) {
			alertsOnly = true;
		}

		if (!target.attr("formaction")) return;

		if (target.hasClass("btn-danger") || target.attr("data-open-confirm") || target.hasClass("delete-button")) {
			var title = target.attr("title");
			if (title !== undefined) {
				if (!confirm("Deleting " + title + ". Are you sure you want to do this?")) {
					event.preventDefault(true);
					event.stopPropagation();
					return;
				}
			} else {
				if (!confirm("Are you sure you want to do this?")) {
					event.preventDefault(true);
					event.stopPropagation();
					return;
				}
			}
		}

		// Find the parent form using the parents or the form attribute
		var parentForm = target.parents("form");
		if (parentForm.length == 0) {
			if (target.attr("form")) {
				parentForm = $("#" + target.attr("form"));
				if (parentForm.length == 0) {
					return;
				}
			} else {
				return
			}
		}

		if (parentForm.attr("data-async-form") === undefined) {
			return;
		}

		event.preventDefault();
		console.log("Should submit using " + target.attr("formaction"), event, parentForm);
		submitForm(parentForm, target.attr("formaction"), alertsOnly);

	});
}

function submitForm(form, url, alertsOnly) {
	var serialized = serializeForm(form);

	if (!alertsOnly) {
		alertsOnly = form.attr("data-async-form-alertsonly") !== undefined;
	}

	// Keep the current tab selected
	var currentTab = null
	var tabElements = $(".tabs");
	if (tabElements.length > 0) {
		currentTab = $(".tabs a.active").attr("href")
	}

	navigate(url, "POST", serialized, false, true, alertsOnly, function () {
		if (currentTab) {
			$(".tabs a[href='" + currentTab + "']").tab("show");
		}
	});

	$.magnificPopup.close();
}

function serializeForm(form) {
	var serialized = form.serialize();

	form.find("[data-content-editable-form]").each(function (i, v) {
		var name = $(v).attr("data-content-editable-form")
		var value = encodeURIComponent($(v).text())
		serialized += "&" + name + "=" + value;
	})

	return serialized
}

function yagInitUnsavedForms(selectorPrefix) {
	let unsavedForms = $(selectorPrefix + "form")
	unsavedForms.each(function (i, rawElem) {
		trackForm(rawElem);
	});
}

function trackForm(form) {
	let savedVersion = serializeForm($(form));

	let hasUnsavedChanges = false

	$(form).change(function () {
		console.log("Form changed!");
		checkForUnsavedChanges();
	})

	var observer = new MutationObserver(function (mutationList, observer) {
		if (!document.body.contains(form)) {
			observer.disconnect();
			hideUnsavedChangesPopup(form);
			return;
		}

		// for (let mutation of mutationList) {
		// 	for (let removed of mutation.removedNodes) {
		// 		if (removed === form) {
		// 			observer.disconnect();
		// 			hideUnsavedChangesPopup(form);
		// 			return
		// 		}
		// 	}
		// }

		if (isSavingUnsavedForms)
			checkForUnsavedChanges();
	});

	observer.observe(document.body, { childList: true, subtree: true });

	function checkForUnsavedChanges() {
		let newVersion = serializeForm($(form));
		if (newVersion !== savedVersion) {
			console.log("Its different!");
			hasUnsavedChanges = true;
			showUnsavedChangesPopup(form);
		} else {
			hasUnsavedChanges = false;
			console.log("It's the same!");
			hideUnsavedChangesPopup(form);
		}
	}
}

let unsavedChangesStack = [];
let isSavingUnsavedForms = false;

function showUnsavedChangesPopup(form) {
	if (unsavedChangesStack.includes(form)) {
		return;
	}

	unsavedChangesStack.push(form)
	updateUnsavedChangesPopup();
}

function hideUnsavedChangesPopup(form) {
	if (!unsavedChangesStack.includes(form)) {
		return;
	}

	let index = unsavedChangesStack.indexOf(form);
	unsavedChangesStack.splice(index, 1);
	updateUnsavedChangesPopup(form)
}

function updateUnsavedChangesPopup() {
	if (unsavedChangesStack.length == 0) {
		$("#unsaved-changes-popup").attr("hidden", true)
	} else {
		if (unsavedChangesStack.length == 1) {
			$("#unsaved-changes-message").text("You have unsaved changes, would you like to save them?");
			if (!isSavingUnsavedForms)
				$("#unsaved-changes-save-button").attr("hidden", false);

		} else {
			$("#unsaved-changes-message").text("You have unsaved changes on multiple forms, save them all?");
			if (!isSavingUnsavedForms)
				$("#unsaved-changes-save-button").attr("hidden", false);

		}

		$("#unsaved-changes-popup").attr("hidden", false)
	}
}

function saveUnsavedChanges() {

	if (unsavedChangesStack.length == 1) {
		let form = unsavedChangesStack[0];
		var action = $(form).attr("action");
		if (!action) {
			action = window.location.pathname;
		}

		submitForm($(form), action, false);
		unsavedChangesStack = [];
		updateUnsavedChangesPopup();
	} else {
		saveNext();
	}

	function saveNext() {
		$("#unsaved-changes-save-button").attr("hidden", true);

		console.log("Saving next");
		let form = unsavedChangesStack.pop();

		let action = $(form).attr("action");
		if (!action) {
			action = window.location.pathname;
		}

		let jf = $(form)
		let serialized = serializeForm(jf);

		// let alertsOnly = jf.attr("data-async-form-alertsonly") !== undefined;
		// if (!alertsOnly) {
		// 	alertsOnly = 
		// }

		// Keep the current tab selected
		// let currentTab = null
		// let tabElements = $(".tabs");
		// if (tabElements.length > 0) {
		// 	currentTab = $(".tabs a.active").attr("href")
		// }

		navigate(action, "POST", serialized, false, true, true, function () {
			console.log("Doneso!");
			if (unsavedChangesStack.length > 0) {
				saveNext();
			} else {
				isSaving = false;
				updateUnsavedChangesPopup();
			}
		});

		$.magnificPopup.close();
	}
}

function navigateToAnchor(name) {
	name = name.substring(1);

	var elem = $("a[name=\"" + name + "\"]");
	if (elem.length < 1) {
		return;
	}

	$('html, body').animate({
		scrollTop: elem.offset().top - 60
	}, 500);

	var offset = elem.offset().top;
	console.log(offset)

	window.location.hash = "#" + name
}

function createRequest(method, path, data, cb) {
	var oReq = new XMLHttpRequest();
	oReq.addEventListener("load", cb);
	oReq.addEventListener("error", function () {
		window.location.href = '/';
	});
	oReq.open(method, path);

	if (data) {
		oReq.setRequestHeader("content-type", "application/json");
		oReq.send(JSON.stringify(data));
	} else {
		oReq.send();
	}
}

function toggleTheme() {
	var elem = document.documentElement;
	if (elem.classList.contains("dark")) {
		elem.classList.remove("dark");
		elem.classList.add("sidebar-light")
		document.cookie = "light_theme=true; max-age=3153600000; path=/"
	} else {
		elem.classList.add("dark");
		elem.classList.remove("sidebar-light")
		document.cookie = "light_theme=false; max-age=3153600000; path=/"
	}
}

function loadWidget(destinationParentID, path) {
	createRequest("GET", path + "?partial=1", null, function () {
		$("#" + destinationParentID).html(this.responseText);
	})
}