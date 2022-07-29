// Copyright © Rob Burke inchworks.com, 2020.
//
// Client-side functions for cards.

jQuery(document).ready(function() {

    // IE 10+ hack to fit images
    $('.ie-image').each(function() {
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