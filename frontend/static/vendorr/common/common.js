// Tooltip and Popover
(function($) {
	$('[data-toggle="tooltip"]').tooltip();
	$('[data-toggle="popover"]').popover();
})(jQuery);

// Tabs
$(document).on('shown.bs.tab', 'a[data-toggle="tab"]', function (e) {
	$(this).parents('.nav-tabs').find('.active').removeClass('active');
	$(this).parents('.nav-pills').find('.active').removeClass('active');
	$(this).addClass('active').parent().addClass('active');
});

// Bootstrap Datepicker
if (typeof($.fn.datepicker) != 'undefined') {
	$.fn.bootstrapDP = $.fn.datepicker.noConflict();
}