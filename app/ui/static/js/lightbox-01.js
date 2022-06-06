// Copyright © Rob Burke inchworks.com, 2022.

// Client-side functions for lokesh/lightbox2.

jQuery(document).ready(function() {

    // Add additional keys to lightbox, to match our slideshows,
    // using keyup because that is what lightbox expects.
    // It checks event.keyCode, but we set event.which as well for consistency.
    $(document).on('keyup', function(e) {

        switch (e.which) {

            case 13: // enter
            case 32: // space
            case 34: // page down
                // next image
                $('.lightbox').trigger($.Event( 'keyup.keyboard', {which:39, keyCode:39}));
                break;

            case 81: // (q)uit
            case 88: // e(x)it
                // end lightbox
                $('.lightbox').trigger($.Event( 'keyup.keyboard', {which:27, keyCode:27}));
                break;

            case 33: // page up
                // previous image
                $('.lightbox').trigger($.Event( 'keyup.keyboard', {which:37, keyCode:37}));
                break;
        }
    })

    // Also, disable default scrolling of window when lightbox is showing.
    $(document).on('keydown', function(e) {
        switch (e.which) {
            case 32: // space
            case 33: // page up
            case 34: // page down
                if ( $('#lightbox').is(":visible") ) {
                    e.preventDefault();
                }
                break;
        }
    })

});

