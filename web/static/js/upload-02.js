// Copyright © Rob Burke inchworks.com, 2021.

// Client-side functions to upload images and videos.

// Upload file specified for uploading.
function uploadFile($inp, token, maxUpload, timestamp, $btnSubmit) {

    var fileName = $inp.val().split("\\").pop();
    var file = $inp[0].files[0];
    var $slide = $inp.closest(".childForm")

    // disable submit button
    $btnSubmit.prop("disabled", true);

    // show file name in form entry, as confirmation to user ..
    $inp.siblings(".custom-file-label").addClass("selected").html(fileName);

    // and in hidden field, so we can match the image to the slide
    $inp.siblings(".imageName").val(fileName);

    // clear previous status
    reset($slide);

    // check file size (rounding to nearest MB)
    var sz = (file.size + (1 << 19)) >> 20
    if (sz > maxUpload) {
         uploadRejected($slide, "This file is " + sz + " MB, " + maxUpload + " MB is allowed");
         return;
    }

    // show progress and status
    $slide.find(".upload").show();

    // upload file with AJAX
    var fd = new FormData();
    fd.append('csrf_token', token);
    fd.append('image', file);

    $.ajax({
        url: '/upload/'+timestamp,  
        type: 'POST',
        data: fd,
        dataType: 'json',
        success:function(reply, rqStatus, jq){ uploaded($slide, reply, rqStatus) },
        error:function(jq, rqStatus, error){ uploadFailed($slide, rqStatus, error) },
        cache: false,
        contentType: false,
        processData: false,
        xhr: function() { return xhrWithProgress($slide); }
    });
}

// XHR object with progress monitoring.
function xhrWithProgress($slide) {
    var xhr = $.ajaxSettings.xhr();
    var $p = $slide.find(".progress-bar");
    xhr.upload.onprogress = function (e) {
        if (e.lengthComputable) {
            var percent = (e.loaded / e.total) * 100;
            $p.width(percent + '%');
        }
    };
    return xhr;	
}

// Event handler for upload request done.
function uploaded($slide, reply, rqStatus) {
    var $alert = $slide.find(".alert")
    if (reply.error == "")
        setStatus($alert, "uploaded", "alert-success");

    else {
        // rejected by server - discard filename
        setStatus($alert, reply.error, "alert-danger");
        $slide.find(".imageName").val("");
    }

    // re-enable submit button
    $("#submit").prop("disabled", false);
}

// Event handler for upload failed.
function uploadFailed($slide, rqStatus, error) {
    var $alert = $slide.find(".alert")
    setStatus($alert, rqStatus + " : " + error, "alert-danger")

    // discard filename, so client doesn't claim to have uploaded it
    $slide.find(".imageName").val("");

    // re-enable submit button
    $("#submit").prop("disabled", false);
}

// Upload rejected.
function uploadRejected($slide, error) {
    var $badFile = $slide.find(".bad-file");
    $badFile.text(error);
    $badFile.show();

    // discard filename, so client doesn't claim to have uploaded it
    $slide.find(".imageName").val("");

    // re-enable submit button
    $("#submit").prop("disabled", false);
}

// Reset upload bar and status fields.
function reset($slide) {
    $slide.find(".upload").hide();
    $slide.find(".progress-bar").width(0);

    var $alert = $slide.find(".alert")
    $alert.text("");
    $alert.removeClass("alert-success alert-danger");

    var $badFile = $slide.find(".bad-file");
    $badFile.text("");
    $badFile.hide();
}

// Set upload status.
function setStatus($alert, status, highlight) {
    $alert.text(status);
    $alert.addClass(highlight);
}
