lastLoc = window.location.pathname;
lastHash = window.location.hash;
$(function(){

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
       	if(window.location.pathname !== lastLoc){
       		shouldNav = true;
       	}else {
       		shouldNav = false;
       	}

		console.log("Popped state", shouldNav, evt, evt.path);
       	if (shouldNav) {
			navigate(window.location.pathname, "GET", null, false)
       	}
        // Handle the back (or forward) buttons here
        // Will NOT handle refresh, use onbeforeunload for this.
    };

    if(window.location.hash){
    	navigateToAnchor(window.location.hash);
    }

   	updateSelectedMenuItem(window.location.pathname);

   	// Update all dropdowns
	// $(".btn-group .dropdown-menu").dropdownUpdate();
})

var currentlyLoading = false;
function navigate(url, method, data, updateHistory, maintainScroll){
    if (currentlyLoading) {return;}
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

	console.log("Navigating to "+url);
	var shownURL = url;
	// Add the partial param
	var index = url.indexOf("?")
	if (index !== -1) {
		url += "&partial=1"	
	}else{
		url += "?partial=1"	
	}

	updateSelectedMenuItem(url);

	var req = new XMLHttpRequest();
    req.addEventListener("load", function(){
    	currentlyLoading = false;
		if (this.status != 200) {
	    	window.location.href = '/';
	    	return;
		}

		$("#main-content").html(this.responseText);
		if (updateHistory) {	
			window.history.pushState("", "", shownURL);
		}
		lastLoc = shownURL;
		lastHash = window.location.hash;
			
		initPlugins(true);
		$(document.body).trigger('ready');
		
		if (typeof ga !== 'undefined') {
			ga('send', 'pageview', window.location.pathname);
			console.log("Sent pageview")
		}

		if(maintainScroll)
			document.documentElement.scrollTop = scrollBeforeNav;

		$("#loading-overlay").addClass("hidden");
    });

    req.addEventListener("error", function(){
        window.location.href = '/';
        currentlyLoading = false;
    });

    req.open(method, url);
    
    if (data) {
        req.setRequestHeader("content-type", "application/x-www-form-urlencoded");
        req.send(data);
    }else{
        req.send();
    }
}

function closeSidebar(){
	document.documentElement.classList.remove("sidebar-left-opened");

	$(window).trigger( "sidebar-left-opened", {
					added: false,
					removed: true
				});
}

// Automatically marks the the menu entry corresponding with our active page as active
function updateSelectedMenuItem(pathname){
	// Collapse all nav parents first
	var navParents = document.querySelectorAll("#menu .nav-parent");
	for(var i = 0; i < navParents.length; i++){
		navParents[i].classList.remove("nav-expanded", "nav-active");
	}

	// Then update the nav links
	var navLinks = document.querySelectorAll("#menu .nav-link")
	for(var i = 0; i < navLinks.length; i++){
		var href = navLinks[i].attributes.getNamedItem("href").value;
		if(pathname.indexOf(href) !== -1){
	
			var collapseParent = navLinks[i].parentElement.parentElement.parentElement;
			if(collapseParent.classList.contains("nav-parent")){
				collapseParent.classList.add("nav-expanded", "nav-active");
			}

			navLinks[i].parentElement.classList.add("nav-active");
		}else{
			navLinks[i].parentElement.classList.remove("nav-active");
		}
	}
}

function addAlert(kind, msg){
	$("<div/>").addClass("row").append(
		$("<div/>").addClass("col-lg-12").append(
			$("<div/>").addClass("alert alert-"+kind).text(msg)
		)
	).appendTo("#alerts");
}

function clearAlerts(){
	$("#alerts").empty();
}

function addListeners(){
	////////////////////////////////////////
	// Async partial page loading handling
	///////////////////////////////////////

	formSubmissionEvents();

	$(document).on("click", '[data-partial-load="true"]', function( event ) {
		console.log("Clicked the link");
		event.preventDefault();
		
	    if (currentlyLoading) {return;}
			
		var link = $(this);
		
		var url = link.attr("href");
		navigate(url, "GET", null, true);
	});

	$(document).on("click", '[data-toggle="popover"]', function(evt){
		$('[data-toggle="popover"]').each(function(i, elem){
			// console.log(elem, elem == evt.target);
			if (evt.currentTarget == elem) {
				return;
			}
			$(elem).popover('hide');
		})
	});

	$(document).on("click", 'a[href^="#"]', function(e) {
	    //e.preventDefault();
    
	    navigateToAnchor($.attr(this, "href"));
 	})
}

// Initializes plugins such as multiselect, we have to do this on the new elements each time we load a partial page
function initPlugins(partial){
	var selectorPrefix = "";
	if (partial) {
		selectorPrefix = "#main-content ";
	}

	$(selectorPrefix + '[data-toggle="popover"]').popover()

	// The uitlity that checks wether the bot has permissions to send messages in the selected channel
	channelRequirepermsDropdown(selectorPrefix);
	yagInitSelect2(selectorPrefix)
	yagInitMultiSelect(selectorPrefix)
	yagInitAutosize(selectorPrefix);
	// initializeMultiselect(selectorPrefix);
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
function channelRequirepermsDropdown(prefix){
	var dropdowns = $(prefix + "select[data-requireperms-send]");
	dropdowns.each(function(i, rawElem){
		trackChannelDropdown($(rawElem), [discordPermissions.read, discordPermissions.send]);
	});

	var dropdownsLinks = $(prefix + "select[data-requireperms-embed]");
	dropdownsLinks.each(function(i, rawElem){
		trackChannelDropdown($(rawElem), [discordPermissions.read, discordPermissions.send, discordPermissions.embed]);
	});
}

function trackChannelDropdown(dropdown, perms){
	var currentElem = $('<p class="form-control-static">Checking channel permissions for bot...</p>');
	dropdown.after(currentElem);

	dropdown.on("change", function(){
		check();
	})

	function check(){
		currentElem.text("Checking channel permissions for bot...");
		currentElem.removeClass("text-success", "text-danger");
		var currentSelected = dropdown.val();
		if(!currentSelected){
			currentElem.text("");
		}else{
			validateChannelDropdown(dropdown, currentElem, currentSelected, perms);
		}
	}
	check();
}

function validateChannelDropdown(dropdown, currentElem, channel, perms){
	// Expire after 5 seconds
	if(cachedChannelPerms[channel] && (!cachedChannelPerms[channel].lastChecked || Date.now() - cachedChannelPerms[channel].lastChecked < 5000)){
		var obj = cachedChannelPerms[channel];
		if(obj.fetching){
			window.setTimeout(function(){
				validateChannelDropdown(dropdown, currentElem, channel, perms);
			}, 1000)
		}else{
			check(cachedChannelPerms[channel].perms);
		}
	}else{
		cachedChannelPerms[channel] = {fetching: true};
		createRequest("GET", "/api/"+CURRENT_GUILDID+"/channelperms/"+channel, null, function(){
			console.log(this);
			cachedChannelPerms[channel].fetching = false;
			if(this.status != 200){
				currentElem.addClass("text-danger");
				currentElem.removeClass("text-success");

				if(this.responseText){
					var decoded = JSON.parse(this.responseText);
					if(decoded.message){
						currentElem.text(decoded.message);
					}else{
						currentElem.text("Couldn't check permissions :(");
					}
				}else{
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

	function check(channelPerms){
		var missing = [];
		for(var i in perms){
			var p = perms[i];
			if((channelPerms&p.perm) != p.perm){
				missing.push(p.name);
			}
		}

		// console.log(missing.join(", "));
		if(missing.length < 1){
			// Has perms
			currentElem.removeClass("text-danger");
			currentElem.addClass("text-success");
			currentElem.text("");
		}else{
			currentElem.addClass("text-danger");
			currentElem.removeClass("text-success");

			currentElem.text("Missing "+missing.join(", "));
		}
	}
}

function initializeMultiselect(selectorPrefix){
	// $(selectorPrefix+".multiselect").multiselect();
}

function formSubmissionEvents(){
	// Form submission fuckery
	$(document).on("submit", "form", submitform);
	
	// forms.each(function(i, elem){
	// 	elem.onsubmit = submitform;
	// })

	function dangerButtonClick(evt){
		var target = $(evt.target);
		if(target.attr("noconfirm") !== undefined){
			return;
		}

		if ($(evt.target).attr("noconfirm")) {
			console.log("no confirm")
			return
		}

		if($(evt.target).attr("formaction")){
			return;
		}

		// console.log("aaaaa", evt, evt.preventDefault);
		if(!confirm("Are you sure you want to do this?")){
			evt.preventDefault(true);
			evt.stopPropagation();
		}
		// alert("aaa")
	} 

	$(document).on("click", ".btn-danger", dangerButtonClick);
	$(document).on("click", ".delete-button", dangerButtonClick);

	

	function submitform(evt){
		for(var i = 0; i < evt.target.elements.length; i++){
			var control = evt.target.elements[i];
			if (control.type === "submit") {
				$(control).addClass("disabled").attr("disabled");
				// var endless = getRandomInt(0, possibilities.length-1)
				// $(control).text(possibilities[endless]);
			}						
		}
	}

	function getRandomInt(min, max) {
		min = Math.ceil(min);
		max = Math.floor(max);
		return Math.floor(Math.random() * (max - min)) + min;
	}


	$(document).on("submit", '[data-async-form]', function( event ) {
		// console.log("Clicked the link");
		event.preventDefault();

	 	var action = $(event.target).attr("action");
	 	if(!action){
	 		action = window.location.pathname;
	 	}
		console.log("Should submit with defualt shizz " + action, event);

	 	submitForm($(event.target), action);
	});

	$(document).on("click", 'button', function( event ) {
		// console.log("Clicked the link");
		var target = $(event.target);

		if(target.hasClass("btn-danger") || target.attr("data-open-confirm") || target.hasClass("delete-button")){
			if(!confirm("Are you sure you want to do this?")){
				event.preventDefault(true);
				event.stopPropagation();
				return;
			}
		}

		// Find the parent form using the parents or the form attribute
		var parentForm = target.parents("form");
		if(parentForm.length == 0){
			if(target.attr("form")){
				parentForm = $("#" + target.attr("form"));
				if(parentForm.length == 0){
					console.log("Not found");
					return;
				}
			}else{
				console.log("Boooo");
				return
			}
		}
		console.log(event);

		if(target.prop("tagName") !== "BUTTON"){
			target = target.parents("button");
			console.log("Not a button!", target);
		}

		if(!target.attr("formaction")) return;

		event.preventDefault();
		console.log("Should submit using " + target.attr("formaction"), event, parentForm);
	 	submitForm(parentForm, target.attr("formaction"));
	
	});

	function submitForm(form, url){
		var serialized = form.serialize();
		console.log(serialized);
		navigate(url, "POST", serialized, false, true);
	}

}

function navigateToAnchor(name){
	name = name.substring(1);

	var elem = $("a[name=\""+name+"\"]");
	if(elem.length < 1){
		return;
	}

    $('html, body').animate({
        scrollTop: elem.offset().top-60
    }, 500);

    var offset = elem.offset().top;
    console.log(offset)

    window.location.hash = "#"+name
}

function createRequest(method, path, data, cb){
    var oReq = new XMLHttpRequest();
    oReq.addEventListener("load", cb);
    oReq.addEventListener("error", function(){
        window.location.href = '/';
    });
    oReq.open(method, path);
    
    if (data) {
        oReq.setRequestHeader("content-type", "application/json");
        oReq.send(JSON.stringify(data));
    }else{
        oReq.send();
    }
}

function toggleTheme(){
	var elem = document.documentElement;
	if(elem.classList.contains("dark")){
		elem.classList.remove("dark");
		elem.classList.add("sidebar-light")
		document.cookie = "X-Light-Theme=true; max-age=3153600000; path=/"
	}else{		
		elem.classList.add("dark");
		elem.classList.remove("sidebar-light")
		document.cookie = "X-Light-Theme=false; max-age=3153600000; path=/"
	}
}