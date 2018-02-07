lastLoc = window.location.pathname;
lastHash = window.location.hash;
$(function(){
	if (visibleURL) {
		console.log("Should navigate to", visibleURL);
		window.history.replaceState("", "", visibleURL);
	}

	addListeners(false);

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
	       	$("#main-content").html('<div class="loader">Loading...</div>');
			navigate(window.location.pathname, "GET", null, false)
       	}
        // Handle the back (or forward) buttons here
        // Will NOT handle refresh, use onbeforeunload for this.
    };

    if(window.location.hash){
    	navigateToAnchor(window.location.hash);
    }

   	// Update all dropdowns
	// $(".btn-group .dropdown-menu").dropdownUpdate();
})

var currentlyLoading = false;
function navigate(url, method, data, updateHistory){
	    if (currentlyLoading) {return;}
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
				

			updateSelectedMenuItem();
			addListeners(true);
			
			if (typeof ga !== 'undefined') {
				ga('send', 'pageview', window.location.pathname);
				console.log("Sent pageview")
			}
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

function addListeners(partial){
	var selectorPrefix = "";
	if (partial) {
		selectorPrefix = "#main-content ";
	}

	////////////////////////////////////////
	// Async partial page loading handling
	///////////////////////////////////////
	$( selectorPrefix + ".nav-link" ).click(function( event ) {
		console.log("Clicked the link");
		event.preventDefault();
		
	    if (currentlyLoading) {return;}
			
		var link = $(this);
		
		var url = link.attr("href");
		navigate(url, "GET", null, true);

		$("#main-content").html('<div class="loader">Loading...</div>');
	});

	initializeMultiselect(selectorPrefix);
	formSubmissionEvents(selectorPrefix);
	channelRequirepermsDropdown(selectorPrefix);
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
	var currentElem = $('<div class="form-group"><p class="text-danger form-control-static">Checking...</p></div>');
	dropdown.parent().after(currentElem);

	dropdown.on("change", function(){
		check();
	})

	function check(){
		currentElem.text("Checking...");
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
						currentElem.text("Something went wrong");
					}
				}else{
					currentElem.text("Something went wrong");
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
			currentElem.text("OK Perms");
		}else{
			currentElem.addClass("text-danger");
			currentElem.removeClass("text-success");

			currentElem.text("Missing "+missing.join(", "));
		}
	}
}

function initializeMultiselect(selectorPrefix){
	$(selectorPrefix+".multiselect").multiselect();
}

function formSubmissionEvents(selectorPrefix){
	// Form submission fuckery
	var forms = $(selectorPrefix + "form");
	
	forms.each(function(i, elem){
		elem.onsubmit = submitform;
	})

	function dangerButtonClick(evt){
		var target = $(evt.target);
		if(target.attr("noconfirm") !== undefined){
			return;
		}

		if ($(evt.target).attr("noconfirm")) {
			console.log("no confirm")
			return
		}

		// console.log("aaaaa", evt, evt.preventDefault);
		if(!confirm("Are you sure you want to do this?")){
			evt.preventDefault(true);
			evt.stopPropagation();
		}
		// alert("aaa")
	} 

	$(selectorPrefix + ".btn-danger").click(dangerButtonClick)
	$(selectorPrefix + ".delete-button").click(dangerButtonClick)

	$(selectorPrefix + '[data-toggle="popover"]').popover()
	$(selectorPrefix + '[data-toggle="popover"]').click(function(evt){
		$('[data-toggle="popover"]').each(function(i, elem){
			// console.log(elem, elem == evt.target);
			if (evt.currentTarget == elem) {
				return;
			}
			$(elem).popover('hide');
		})
	})

	function submitform(evt){

		var possibilities = [
			"Loading!",
			"Loading...",
			"Loading :D",
			"Not Loading!",
			"Unloading!",
			"Burning down your house!",
			"Having a bbq!",
			"Taking it slow...",
			"Snack break!",
			"How was your day?",
			"HAHAHAHAHA Are you fast enough to read his? you're an idiot.",
			"Yo listen up here's a story, About a little guy that lives in a blue world, And all day and all night and everything he sees, Is just blue like him inside and outside, Blue his house with a blue little window, And a blue corvette, And everything is blue for him and himself, And everybody around, 'Cause he ain't got nobody to listen to",
			"Am loading sir o7",
			"Wonder what this button does?",
			"Hmmm this sure is taking a long time",
			"Wanna go out on a date?",
			"Am I loading?",
			"Click me harder boi ;)	)))",
			"OwO Whats this?",
			"I think I broke it!",
			"Oh hello there!",
			"Now you see me, now you dont.",
			"Erorr 404: Response not found.",
			"Wait... You're actually reading this?",
			"Maybe if I just wait a little longer...",
		]

		for(var i = 0; i < evt.target.elements.length; i++){
			var control = evt.target.elements[i];
			if (control.type === "submit") {
				$(control).addClass("disabled").attr("disabled");
				var endless = getRandomInt(0, possibilities.length-1)
				$(control).text(possibilities[endless]);
			}						
		}
	}

	function getRandomInt(min, max) {
		min = Math.ceil(min);
		max = Math.floor(max);
		return Math.floor(Math.random() * (max - min)) + min;
	}

	const $navbar = $('.navbar');
	$(selectorPrefix + 'a[href^="#"]').on('click', function(e) {
	    e.preventDefault();
    
	    navigateToAnchor($.attr(this, "href"));
 
	    // e.target.scrollIntoView({"behaviour": "smooth", "block": "end"});
	    // const scrollTop =
	    //     $(e).position().top -
	    //     $navbar.outerHeight();

	    // $('html, body').animate({ scrollTop });
	})
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
