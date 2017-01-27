	$(function(){
	var forms = $("form");
	
	forms.each(function(i, elem){
		elem.onsubmit = submitform;
	})

	$(".btn-danger").click(function(evt){
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
	})

	$('[data-toggle="popover"]').popover()
	$('[data-toggle="popover"]').click(function(evt){
		$('[data-toggle="popover"]').each(function(i, elem){
			// console.log(elem, elem == evt.target);
			if (evt.currentTarget == elem) {
				return;
			}
			$(elem).popover('hide');
		})
	})

	console.log("aa");

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
			"Am i loading?",
			"Click me harder boi ;)	)))",
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

	if (visibleURL) {
		console.log("Should navigate to", visibleURL);
		window.history.pushState("", "", visibleURL);
	}
})


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