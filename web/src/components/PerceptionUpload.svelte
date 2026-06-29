<!-- e:\opsec-control\web\src\components\PerceptionUpload.svelte -->
<script>
  import { createEventDispatcher } from 'svelte';
  
  var dispatch = createEventDispatcher();
  
  var selectedSource = "drone";
  var dragActive = false;
  var uploadState = "idle"; // "idle", "uploading", "analyzing", "success", "error"
  var statusMessage = "";
  var thumbnail = null;
  var fileInput;

  // Preset triggers for quick MVD mock testing
  var presets = [
    { name: "Vora Bridge Collapse", data: "drone_vora_bridge_collapsed.png", source: "drone" },
    { name: "Highgate Collapse", data: "satellite_highgate_masonry_collapse.png", source: "satellite" },
    { name: "Southport Levee Breach", data: "drone_southport_levee_breach.png", source: "drone" }
  ];

  function handleDrag(e) {
    e.preventDefault();
    e.stopPropagation();
    if (e.type === "dragenter" || e.type === "dragover") {
      dragActive = true;
    } else if (e.type === "dragleave") {
      dragActive = false;
    }
  }

  function handleDrop(e) {
    e.preventDefault();
    e.stopPropagation();
    dragActive = false;
    
    if (e.dataTransfer.files && e.dataTransfer.files[0]) {
      processFile(e.dataTransfer.files[0]);
    }
  }

  function handleFileSelect(e) {
    if (e.target.files && e.target.files[0]) {
      processFile(e.target.files[0]);
    }
  }

  function triggerFileInput() {
    fileInput.click();
  }

  // Create local preview thumbnail for snazzy UX
  function processFile(file) {
    if (file.size > 10 * 1024 * 1024) {
      uploadState = "error";
      statusMessage = "Payload too large (max 10MB)";
      dispatch('error', statusMessage);
      return;
    }

    var reader = new FileReader();
    reader.onload = function(e) {
      thumbnail = e.target.result;
    };
    reader.readAsDataURL(file);
    
    uploadImage(file);
  }

  async function uploadImage(file) {
    uploadState = "uploading";
    statusMessage = "Uploading image...";
    dispatch('uploading');

    try {
      var formData = new FormData();
      formData.append("image", file);
      formData.append("source", selectedSource);

      var res = await fetch("/perception", {
        method: "POST",
        body: formData
      });

      if (!res.ok) {
        var errText = await res.text();
        throw new Error(errText || `Server responded with status ${res.status}`);
      }

      var data = await res.json();
      uploadState = "analyzing";
      statusMessage = "Vision cells perceiving...";
      
      // Wait briefly for snazzy visual transition
      setTimeout(() => {
        uploadState = "success";
        statusMessage = `Triggered ${data.accepted || 0} events!`;
        dispatch('events', data.events || []);
        
        // Reset to idle after 4 seconds
        setTimeout(() => {
          if (uploadState === "success") {
            uploadState = "idle";
            statusMessage = "";
            thumbnail = null;
          }
        }, 4000);
      }, 1200);

    } catch (err) {
      uploadState = "error";
      statusMessage = err.message || "Failed to parse image";
      dispatch('error', statusMessage);
    }
  }

  // Preset upload triggers a small text file post mimicking the PNG trigger content
  async function triggerPreset(preset) {
    uploadState = "uploading";
    statusMessage = `Activating ${preset.name}...`;
    thumbnail = null;
    dispatch('uploading');

    try {
      var res = await fetch(`/perception?source=${preset.source}`, {
        method: "POST",
        headers: { "Content-Type": "application/octet-stream" },
        body: preset.data
      });

      if (!res.ok) {
        var errText = await res.text();
        throw new Error(errText || `Server error ${res.status}`);
      }

      var data = await res.json();
      uploadState = "analyzing";
      statusMessage = "Vision cells perceiving...";
      
      setTimeout(() => {
        uploadState = "success";
        statusMessage = "Events published successfully!";
        dispatch('events', data.events || []);
        
        setTimeout(() => {
          if (uploadState === "success") {
            uploadState = "idle";
            statusMessage = "";
          }
        }, 4000);
      }, 1000);

    } catch (err) {
      uploadState = "error";
      statusMessage = err.message;
      dispatch('error', statusMessage);
    }
  }
</script>

<div class="control-panel upload-panel">
  <div class="panel-title" style="display: flex; justify-content: space-between; align-items: center;">
    <span>Tactical Perception Ingest</span>
    <span class="pulse-indicator" class:active={uploadState !== 'idle'}></span>
  </div>

  <!-- Source Toggle Selector -->
  <div class="source-toggle">
    <button class="source-btn" class:active={selectedSource === 'drone'} on:click={() => selectedSource = 'drone'}>
      <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 16V8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l-7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16z"></path><polyline points="3.27 6.96 12 12.01 20.73 6.96"></polyline><line x1="12" y1="22.08" x2="12" y2="12"></line></svg>
      DRONE
    </button>
    <button class="source-btn" class:active={selectedSource === 'satellite'} on:click={() => selectedSource = 'satellite'}>
      <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"></circle><path d="M12 2a15.3 15.3 0 0 1 4 10 15.3 15.3 0 0 1-4 10 15.3 15.3 0 0 1-4-10 15.3 15.3 0 0 1 4-10z"></path><path d="M2 12h20"></path></svg>
      SATELLITE
    </button>
  </div>

  <!-- Drag and Drop Dropzone -->
  <div 
    class="upload-dropzone" 
    class:drag-active={dragActive} 
    class:uploading={uploadState === 'uploading' || uploadState === 'analyzing'}
    class:success={uploadState === 'success'}
    class:error={uploadState === 'error'}
    on:dragenter={handleDrag}
    on:dragover={handleDrag}
    on:dragleave={handleDrag}
    on:drop={handleDrop}
    on:click={triggerFileInput}
  >
    <input 
      type="file" 
      accept="image/*" 
      style="display: none;" 
      bind:this={fileInput} 
      on:change={handleFileSelect}
    />

    {#if thumbnail}
      <div class="thumbnail-preview">
        <img src={thumbnail} alt="Upload preview" />
      </div>
    {:else if uploadState === 'idle'}
      <div class="upload-icon-wrap">
        <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4M17 8l-5-5-5 5M12 3v12"></path></svg>
      </div>
      <span class="upload-text-primary">Drag & Drop Image</span>
      <span class="upload-text-secondary">or click to browse local files</span>
    {:else if uploadState === 'uploading'}
      <div class="upload-loader"></div>
      <span class="upload-text-primary active">{statusMessage}</span>
    {:else if uploadState === 'analyzing'}
      <div class="analyzing-scanner"></div>
      <span class="upload-text-primary active">{statusMessage}</span>
    {:else if uploadState === 'success'}
      <div class="success-checkmark">
        <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="#00ff87" stroke-width="3"><polyline points="20 6 9 17 4 12"></polyline></svg>
      </div>
      <span class="upload-text-primary active" style="color: #00ff87;">{statusMessage}</span>
    {:else if uploadState === 'error'}
      <div class="error-cross">
        <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="#ff3333" stroke-width="3"><line x1="18" y1="6" x2="6" y2="18"></line><line x1="6" y1="6" x2="18" y2="18"></line></svg>
      </div>
      <span class="upload-text-primary active" style="color: #ff3333; font-size: 0.75rem;">{statusMessage}</span>
    {/if}
  </div>

  <!-- Preset Trigger Buttons -->
  <div class="preset-title">Or trigger quick test presets:</div>
  <div class="preset-grid">
    {#each presets as p}
      <button 
        class="preset-btn" 
        disabled={uploadState === 'uploading' || uploadState === 'analyzing'}
        on:click|stopPropagation={() => triggerPreset(p)}
      >
        {p.name}
      </button>
    {/each}
  </div>
</div>
