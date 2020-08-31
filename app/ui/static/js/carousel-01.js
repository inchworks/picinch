// Copyright © Rob Burke inchworks.com, 2020.

// Client-side functions for slideshow using a Bootstrap carousel.

jQuery(document).ready(function() {

    // keyboard input
    $(document).on('keyup', function(e){
        // hide buttons when keys are used
        if (gblButtons) {
            $('#slideshow1').find('.slideshow-button').hide();
            gblButtons = false;
        }

       switch (e.which) {

            case 13: // enter
            case 32: // space
            case 34: // page down
            case 39: // right arrow
                // next slde
                $('.carousel').carousel('next');
                break;

            case 27: // escape
            case 81: // (q)uit
            case 88: // e(x)it
                // end slideshow
                window.location.href = gblParent;
                break;

            case 33: // page up
            case 37: // left arrow
                // previous slide
                $('.carousel').carousel('prev');
                break;

            case 35: // end
                // last slide
                break;

            case 36: // home
                // first slide
                $('.carousel').carousel(0);
                break;
        }

    });

    // carousel events
    // also gets e.relatedTarget
    $('#slideshow1').on('slide.bs.carousel', function (e) {
        if (e.from === 0 && e.direction === 'right') {
            window.location.href = gblBefore;
        }
        else if (e.to === 0 && e.direction === 'left') {
            window.location.href = gblAfter;
        }
    });

    // quit button
    $('.slideshow-control-quit').on('click', function() {
        window.location.href = gblParent;
    });

    // set first slide active
    $('#slideshow1').find('.carousel-item').first().addClass('active');

    // activate carousel, otherwise swipes don't work until first click
    $('#slideshow1').carousel({
        interval: false
      });

    // IE 10+ hack to fit images
    $('.shrink-image').each(function() {
        objectFit(this);    
    });
    
 });

 // IE hack - move image to background (that supports object-fit) and overlay with transparent SVG image of the same size
 // Thanks to https://www.stevefenton.co.uk/2019/09/fixing-css-object-fit-for-internet-explorer/

 function objectFit(image) {
    if ('objectFit' in document.documentElement.style === false && image.currentStyle['object-fit']) {
        image.style.background = 'url("' + image.src + '") no-repeat 50%/' + image.currentStyle['object-fit'];
        image.src = "data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='" + image.width + "' height='" + image.height + "'%3E%3C/svg%3E";
    }
}