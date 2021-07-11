// Copyright © Rob Burke inchworks.com, 2021.

// Client-side functions to upload images and videos.

// Upload file specified for uploading.
function uploadFile($inp, token, timestamp, $btnSubmit) {

    var fileName = $inp.val().split("\\").pop();

    // disable submit button
    $btnSubmit.prop("disabled", true);

    // in form entry, as confirmation to user ..
    $inp.siblings(".custom-file-label").addClass("selected").html(fileName);

    // and in hidden field, so we can match the image to the slide
    $inp.siblings(".imageName").val(fileName);

    // clear previous status
    reset($inp);

    // show progress and status
    $inp.closest(".childForm").find(".upload").show();

    // upload file with AJAX
    var fd = new FormData();
    fd.append('csrf_token', token);
    fd.append('image', $inp[0].files[0]);

    $.ajax({
        url: '/upload/'+timestamp,  
        type: 'POST',
        data: fd,
        dataType: 'json',
        success:function(reply, rqStatus, jq){ uploaded($inp, reply, rqStatus) },
        error:function(jq, rqStatus, error){ uploadFailed($inp, rqStatus, error) },
        cache: false,
        contentType: false,
        processData: false,
        xhr: function() { return xhrWithProgress($inp); }
    });
}

// XHR object with progress monitoring.
function xhrWithProgress($inp) {
    var xhr = $.ajaxSettings.xhr();
    var $p = $inp.closest(".childForm").find(".progress-bar");
    xhr.upload.onprogress = function (e) {
        if (e.lengthComputable) {
            var percent = (e.loaded / e.total) * 100;
            $p.width(percent + '%');
        }
    };
    return xhr;	
}

// Event handler for upload request done.
function uploaded($inp, reply, rqStatus) {
    var $alert = $inp.closest(".childForm").find(".alert")
    if (reply.error == "")
        setStatus($alert, "uploaded", "alert-success");

    else {
        // rejected by server - discard filename
        setStatus($alert, reply.error, "alert-danger");
        $inp.siblings(".imageName").val("");
    }

    // re-enable submit button
    $("#submit").prop("disabled", false);
}

// Event handler for upload failed.
function uploadFailed($inp, rqStatus, error) {
    var $alert = $inp.closest(".childForm").find(".alert")
    setStatus($alert, rqStatus + " : " + error, "alert-danger")

    // discard filename, so client doesn't claim to have uploaded it
    $inp.siblings(".imageName").val("");

    // re-enable submit button
    $("#submit").prop("disabled", false);
}

// Reset upload bar and status.
function reset($inp) {
    $inp.closest(".childForm").find(".progress-bar").width(0);

    var $alert = $inp.closest(".childForm").find(".alert")
    $alert.text("");
    $alert.removeClass("alert-success alert-danger");
}

// Set upload status.
function setStatus($alert, status, highlight) {
    $alert.text(status);
    $alert.addClass(highlight);
}
