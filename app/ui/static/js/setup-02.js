// Copyright © Rob Burke inchworks.com, 2020.

// Client-side functions.

// Add and remove sub-forms from lists of items. It assumes only one set of sub-forms per page.
// Based on Symfony Cookbook "How to Embed a Collection of Forms", generalised as much I could.
//
// Supports optional confirmation of deletions.

var $collectionHolder;
var $prototype;

jQuery(document).ready(function() {

     // Get the div that holds the collection of items
    $collectionHolder = $('#formChildren');

    // prototype sub-form is the first one
    $prototype = $collectionHolder.find('div').first();

    // handle delete button
    $('.btnDeleteChild').on('click', function(evt) {

       // prevent the link from creating a "#" on the URL
       evt.preventDefault();

       // remove the div for the deleted item
       $(this).closest('.childForm').remove();
    });

    // handle delete with confirmation
    $('.btnConfirmDelChild').on('click', function(evt)	{
      confirmDelete($(this).closest('.childForm'), evt);
    });

    $('.btnAddChild').on('click', function(evt) {
        // prevent the link from creating a "#" on the URL
        evt.preventDefault();

        // add a new child form
        addChildForm($collectionHolder);
    });
 
    // add any page-specific processing
    pageReady();
 });

function addChildForm($collectionHolder) {

    // clone the prototype
    var $newForm = $prototype.clone();

	  // add change handlers (not copied with prototype, it seems)
	  $newForm.find('.btnDeleteChild').on('click', function(evt) {
        // prevent the link from creating a "#" on the URL
        evt.preventDefault();

        // remove the div for the deleted item
        $(this).closest('.childForm').remove();
    });

    // handle delete with confirmation
    $newForm.find('.btnConfirmDelChild').on('click', function(evt)	{
        confirmDelete($(this).closest('.childForm'), evt);
    });

    // hide any buttons that need child to exist
    $newForm.find('.notOnNew').hide();

    // do any page-specific processing
    childAdded($prototype, $newForm);

    // display the form in the page, after the final one
    var $prev = $collectionHolder.children().last();
    $collectionHolder.append($newForm);

    // set the index, needed so that form shows when redisplayed on error, and for checkbox values
    var newIx = Number($prev.find('input[name="index"]').val()) + 1
    $newForm.find('input[name="index"]').val(newIx)

		// set value of any checkboxes to the child index
		$newForm.find(':checkbox').val(newIx)

    // make form visible
    $newForm.css('display', 'block');
	
    return $newForm;
}

// confirm deletion

function confirmDelete($child, evt) {

		var callback = function() {
   			evt.preventDefault();

   			// remove the div for the deleted item
   			$child.remove();
		};

	  confirm(confirmAsk($child), 'Cancel', 'Confirm', callback);
}

// Modal confirmation dialog
// From https://stackoverflow.com/questions/8982295/confirm-deletion-in-modal-dialog-using-twitter-bootstrap/10124151#10124151

function confirm(ask, cancelButtonTxt, okButtonTxt, callback) {

    var confirmModal = 
      $('<div class="modal fade">' +        
          '<div class="modal-dialog">' +
          '<div class="modal-content">' +

          '<div class="modal-body">' +
            '<p>' + ask + '</p>' +
          '</div>' +

          '<div class="modal-footer">' +
            '<a href="#!" class="btn" data-dismiss="modal">' + 
              cancelButtonTxt + 
            '</a>' +
            '<a href="#!" id="okButton" class="btn btn-primary">' + 
              okButtonTxt + 
            '</a>' +
          '</div>' +
          '</div>' +
          '</div>' +
        '</div>');

    confirmModal.find('#okButton').click(function(event) {
        callback();
        confirmModal.modal('hide');
    }); 

    confirmModal.modal('show');    
}