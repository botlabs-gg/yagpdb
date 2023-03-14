/* Add here all your JS customizations */

/* This one is meant for tab-indent to be active in all textarea if marked tags; does not feature undo */
$(document).delegate('.tab-textbox, textarea.form-control', 'keydown', function(e) { 
var keyCode = e.keyCode || e.which; 

    if (keyCode == 9) {
      e.preventDefault();
      var start = $(this).get(0).selectionStart;
      var end = $(this).get(0).selectionEnd;

      // caret replacement
      $(this).val($(this).val().substring(0, start)
                  + "\t"
                  + $(this).val().substring(end));

  // caret back
  $(this).get(0).selectionStart = 
  $(this).get(0).selectionEnd = start + 1;
  onCCChanged(this);
} 
});
