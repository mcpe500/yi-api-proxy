/**
 * Media Handler - Process images for AI providers
 * 
 * Handles:
 * - Vision model detection (auto-detect provider capability)
 * - Image to text conversion (when non-vision provider receives images)
 * 
 * Providers:
 * - GLM (GLM-5.1): Supports vision
 * - MiniMax (M2.7): NO vision - images auto-converted via GLM
 * - Z.ai: Supports vision (Claude models)
 */

/**
 * Check if request body contains images
 */
export function hasImages(body) {
  if (!body) return false;
  
  // OpenAI compatible format with messages
  if (body.messages) {
    for (const msg of body.messages) {
      if (msg.content && Array.isArray(msg.content)) {
        for (const content of msg.content) {
          if (content.type === 'image_url' || content.type === 'input_image') {
            return true;
          }
        }
      }
    }
  }
  
  // Direct content array (Anthropic format)
  if (body.content && Array.isArray(body.content)) {
    return body.content.some(c => 
      c.type === 'image' || 
      c.type === 'image_url' || 
      c.type === 'input_image'
    );
  }
  
  return false;
}

/**
 * Check if provider supports vision for given model
 */
export function providerSupportsVision(provider, model, config) {
  const cfg = config.providers[provider];
  if (!cfg) return false;
  
  // Check if model is in vision models list
  if (cfg.visionModels?.some(m => model?.startsWith(m))) {
    return true;
  }
  
  // Check capabilities flag
  if (cfg.capabilities?.includes('vision')) {
    return true;
  }
  
  return false;
}

/**
 * Get vision-capable provider from config
 */
export function getVisionProvider(config) {
  return config.visionFallback?.provider || 'glm';
}

/**
 * Convert image URL to base64 data URL
 */
export async function imageUrlToBase64(imageUrl) {
  if (!imageUrl) return null;
  
  // Already base64 data URL
  if (imageUrl.startsWith('data:')) {
    return imageUrl;
  }
  
  // HTTP(S) URL - fetch and convert
  if (imageUrl.startsWith('http')) {
    try {
      const response = await fetch(imageUrl);
      if (!response.ok) throw new Error(`HTTP ${response.status}`);
      
      const buffer = await response.arrayBuffer();
      const mimeType = response.headers.get('content-type') || 'image/jpeg';
      const base64 = Buffer.from(buffer).toString('base64');
      return `data:${mimeType};base64,${base64}`;
    } catch (error) {
      console.error('Failed to fetch image:', error.message);
      return null;
    }
  }
  
  // File path
  if (imageUrl.startsWith('file://')) {
    try {
      const { readFileSync } = await import('fs');
      const filePath = imageUrl.replace('file://', '');
      const buffer = readFileSync(filePath);
      return `data:image/jpeg;base64,${buffer.toString('base64')}`;
    } catch (error) {
      console.error('Failed to read file:', error.message);
      return null;
    }
  }
  
  // Assume raw base64 string
  return `data:image/jpeg;base64,${imageUrl}`;
}

/**
 * Use vision model to describe an image
 */
async function describeImage(visionConfig, base64Data, imageIndex = 1) {
  const visionModel = visionConfig.visionModels?.[0] || 'glm-5.1';
  
  const response = await fetch(visionConfig.baseUrl + visionConfig.endpoint, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${visionConfig.apiKey}`
    },
    body: JSON.stringify({
      model: visionModel,
      messages: [{
        role: 'user',
        content: [
          { type: 'image_url', image_url: { url: base64Data } },
          { type: 'text', text: 'Describe this image in detail. Include all objects, text, colors, and notable features. Be specific.' }
        ]
      }],
      max_tokens: 1000
    })
  });
  
  if (!response.ok) {
    const error = await response.text();
    throw new Error(`Vision model error: ${response.status} - ${error}`);
  }
  
  const data = await response.json();
  const description = data.choices?.[0]?.message?.content || '';
  
  return `[Image ${imageIndex}]: ${description}`;
}

/**
 * Extract all images from request body
 */
export function extractImages(body) {
  const images = [];
  
  // Handle messages array (OpenAI/Anthropic format)
  if (body.messages) {
    for (const msg of body.messages) {
      if (!msg.content || !Array.isArray(msg.content)) continue;
      
      for (let i = 0; i < msg.content.length; i++) {
        const content = msg.content[i];
        
        if (content.type === 'image_url' || content.type === 'input_image' || content.type === 'image') {
          const url = content.image_url?.url || content.url || content.source || content.data;
          if (url) {
            images.push({ url, index: images.length + 1, contentIndex: i });
          }
        }
      }
    }
  }
  
  // Handle direct content array (Anthropic format)
  if (body.content && Array.isArray(body.content)) {
    for (let i = 0; i < body.content.length; i++) {
      const content = body.content[i];
      
      if (content.type === 'image' || content.type === 'image_url') {
        const url = content.source || content.url || content.data;
        if (url) {
          images.push({ url, index: images.length + 1, contentIndex: i });
        }
      }
    }
  }
  
  return images;
}

/**
 * Process images - convert to text descriptions using vision model
 * Returns: { modifiedBody, visionProvider }
 */
export async function processImages(body, config, currentModel) {
  const images = extractImages(body);
  
  if (images.length === 0) {
    return { modifiedBody: null, visionProvider: null };
  }
  
  // Get vision provider
  const visionProvider = getVisionProvider(config);
  const visionConfig = config.providers[visionProvider];
  
  if (!visionConfig) {
    console.error('No vision provider configured');
    return { modifiedBody: null, visionProvider: null };
  }
  
  // Convert all images to descriptions
  const descriptions = [];
  
  for (const img of images) {
    try {
      const base64 = await imageUrlToBase64(img.url);
      if (base64) {
        const description = await describeImage(visionConfig, base64, img.index);
        descriptions.push(description);
      }
    } catch (error) {
      console.error(`Failed to process image ${img.index}:`, error.message);
      descriptions.push(`[Image ${img.index}]: [Description unavailable]`);
    }
  }
  
  if (descriptions.length === 0) {
    return { modifiedBody: null, visionProvider: null };
  }
  
  const descriptionsText = descriptions.join('\n\n');
  
  // Modify body to replace images with text descriptions
  const modifiedBody = JSON.parse(JSON.stringify(body)); // Deep clone
  
  // Replace images with descriptions in messages
  if (modifiedBody.messages) {
    for (const msg of modifiedBody.messages) {
      if (!msg.content || !Array.isArray(msg.content)) continue;
      
      const newContent = [];
      
      for (const content of msg.content) {
        if (content.type === 'image_url' || content.type === 'input_image' || content.type === 'image') {
          // Skip image, description added below
          continue;
        } else if (content.type === 'text') {
          // Prepend image descriptions to text
          newContent.push({
            type: 'text',
            text: `[I have analyzed ${images.length} image(s) provided. Here are their descriptions:]\n\n${descriptionsText}\n\nBased on the above, ${content.text || ''}`
          });
        } else {
          newContent.push(content);
        }
      }
      
      // If no text content found, add description as only content
      if (newContent.length === 0) {
        newContent.push({
          type: 'text',
          text: `[I have analyzed ${images.length} image(s) provided. Here are their descriptions:]\n\n${descriptionsText}\n\nPlease answer based on the images described above.`
        });
      }
      
      msg.content = newContent;
    }
  }
  
  // Handle direct content array (Anthropic format)
  if (modifiedBody.content && Array.isArray(modifiedBody.content)) {
    const newContent = [];
    
    for (const content of modifiedBody.content) {
      if (content.type === 'image' || content.type === 'image_url') {
        continue;
      } else if (content.type === 'text') {
        newContent.push({
          type: 'text',
          text: `[I have analyzed ${images.length} image(s). Here are descriptions:]\n\n${descriptionsText}\n\n${content.text}`
        });
      } else {
        newContent.push(content);
      }
    }
    
    if (newContent.length === 0) {
      newContent.push({
        type: 'text',
        text: `[I have analyzed ${images.length} image(s). Here are descriptions:]\n\n${descriptionsText}\n\nPlease answer.`
      });
    }
    
    modifiedBody.content = newContent;
  }
  
  return { modifiedBody, visionProvider };
}

/**
 * Check if images can be processed (vision provider available)
 */
export function canProcessImages(config) {
  const visionProvider = getVisionProvider(config);
  const visionConfig = config.providers[visionProvider];
  return visionConfig && (visionConfig.visionModels?.length > 0);
}

export default {
  hasImages,
  providerSupportsVision,
  getVisionProvider,
  extractImages,
  processImages,
  canProcessImages
};