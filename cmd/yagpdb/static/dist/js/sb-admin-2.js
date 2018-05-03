$(function() {
    console.log("Lemonade tasts good");
    $('#side-menu').metisMenu();
    $('[data-toggle="tooltip"]').tooltip()
});

//Loads the correct sidebar on window load,
//collapses the sidebar on window resize.
// Sets the min-height of #page-wrapper to window size
$(function() {
    $(window).bind("load resize", function() {
        topOffset = 50;
        width = (this.window.innerWidth > 0) ? this.window.innerWidth : this.screen.width;
        if (width < 768) {
            $('div.navbar-collapse').addClass('collapse');
            topOffset = 100; // 2-row-menu
        } else {
            $('div.navbar-collapse').removeClass('collapse');
        }

        height = ((this.window.innerHeight > 0) ? this.window.innerHeight : this.screen.height) - 1;
        height = height - topOffset;
        if (height < 1) height = 1;
        if (height > topOffset) {
            $("#page-wrapper").css("min-height", (height) + "px");
        }
    });

    $('#tabs a').click(function (e) {
        e.preventDefault()
        $(this).tab('show')
    })

    updateSelectedMenuItem();
});

function updateSelectedMenuItem(){
    $("ul.nav a").each(function(i, v){
        var elem = $(v).removeClass("active").parent().parent().removeClass("in").parent();
        if(elem.is("li")){
            elem.removeClass("active");
        }
    });

    var url = window.location;
    var element = $('ul.nav a').filter(function() {
        return this.href == url || url.href.indexOf(this.href) == 0;
    }).addClass('active').parent().parent().addClass('in').parent();
    if (element.is('li')) {
        element.addClass('active');
    }


}

// Adapted from https://bootsnipp.com/snippets/featured/dynamic-form-fields-add-amp-remove-bs3
$(function()
{
    $(document).on('click', '.btn-add', function(e)
    {
        e.preventDefault();

        var currentEntry = $(this).parent().parent(),
            newEntry = $(currentEntry.clone()).insertAfter(currentEntry);

        newEntry.find('input, textarea').val('');
        newEntry.parent().find('.entry:not(:last-of-type) .btn-add')
            .removeClass('btn-add').addClass('btn-remove')
            .removeClass('btn-success').addClass('btn-danger')
            .html('<span class="glyphicon glyphicon-minus"></span>');
    }).on('click', '.btn-remove', function(e)
    {
    $(this).parents('.entry:first').remove();

    e.preventDefault();
    return false;
  });
});
$(function() {
    $(window).bind("load", function() {
      $('.entry:not(:last-of-type) .btn-add')
          .removeClass('btn-add').addClass('btn-remove')
          .removeClass('btn-success').addClass('btn-danger')
          .html('<span class="glyphicon glyphicon-minus"></span>');
    });
});
